package veilroute

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recorded struct {
	path    string
	headers http.Header
	body    []byte
}

func server(t *testing.T, handler func(r recorded) (int, string, http.Header)) (*httptest.Server, *[]recorded) {
	t.Helper()
	var mu sync.Mutex
	calls := &[]recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec := recorded{path: r.URL.Path, headers: r.Header.Clone(), body: b}
		mu.Lock()
		*calls = append(*calls, rec)
		mu.Unlock()
		status, body, h := handler(rec)
		for k, v := range h {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

func clearEnv(t *testing.T) {
	for _, k := range []string{"VEILROUTE_KEY", "VEILROUTE_API_KEY", "VEILROUTE_WEB_KEY", "VEILROUTE_BMP_KEY", "VEILROUTE_WEB_URL", "VEILROUTE_BMP_URL"} {
		t.Setenv(k, "")
	}
}

func newTest(srvURL string, opts ...Option) *Client {
	base := []Option{WithWebBaseURL(srvURL), WithBMPBaseURL(srvURL + "/bmp")}
	c := NewClient(append(base, opts...)...)
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c
}

func TestWebSensor(t *testing.T) {
	clearEnv(t)
	srv, calls := server(t, func(recorded) (int, string, http.Header) {
		return 200, `{"status":"success","sensor":"3;0;1;abc","index":1,"session_id":"s1","ms":210}`, nil
	})
	c := newTest(srv.URL, WithAPIKey("k_web"))
	out, err := c.Web.Sensor(context.Background(), WebSensorParams{
		URL: "https://www.example.com/", UserAgent: "UA/1", Cookies: "_abck=x; bm_sz=y", Index: 1, SessionID: "s1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Sensor != "3;0;1;abc" || out.Index != 1 || out.SessionID != "s1" || out.Ms != 210 || out.Raw["ms"] == nil {
		t.Fatalf("unexpected result: %+v", out)
	}
	r := (*calls)[0]
	if r.path != "/api/v1/sensor" || r.headers.Get("X-Api-Key") != "k_web" || !strings.HasPrefix(r.headers.Get("User-Agent"), "veilroute-go/") {
		t.Fatalf("unexpected request: %s %v", r.path, r.headers)
	}
	var body map[string]any
	_ = json.Unmarshal(r.body, &body)
	want := map[string]any{"url": "https://www.example.com/", "userAgent": "UA/1", "index": float64(1), "session_id": "s1", "cookies": "_abck=x; bm_sz=y"}
	if len(body) != len(want) {
		t.Fatalf("body = %v", body)
	}
	for k, v := range want {
		if body[k] != v {
			t.Fatalf("body[%s] = %v, want %v", k, body[k], v)
		}
	}
}

func TestIndexZeroIsAlwaysSent(t *testing.T) {
	clearEnv(t)
	srv, calls := server(t, func(recorded) (int, string, http.Header) { return 200, `{"sensor":"s"}`, nil })
	c := newTest(srv.URL, WithAPIKey("k"))
	if _, err := c.Web.Sensor(context.Background(), WebSensorParams{URL: "https://e.com/", UserAgent: "UA"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string((*calls)[0].body), `"index":0`) {
		t.Fatalf("index missing: %s", (*calls)[0].body)
	}
}

func TestBMPSensorUsesItsOwnKey(t *testing.T) {
	clearEnv(t)
	srv, calls := server(t, func(recorded) (int, string, http.Header) {
		return 200, `{"sensor":"1.70~4~x","deviceId":"d1","platform":"android","android":"14","sdkVersion":"1.0.0","latencyMs":42}`, nil
	})
	c := newTest(srv.URL, WithWebAPIKey("k_web"), WithBMPAPIKey("k_bmp"))
	out, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "com.example.app"})
	if err != nil {
		t.Fatal(err)
	}
	r := (*calls)[0]
	if r.path != "/bmp/api/v1/sensor" || r.headers.Get("X-Api-Key") != "k_bmp" || string(r.body) != `{"app":"com.example.app"}` {
		t.Fatalf("unexpected request: %s %s %s", r.path, r.headers.Get("X-Api-Key"), r.body)
	}
	if out.DeviceID != "d1" || out.SDKVersion != "1.0.0" || out.LatencyMs != 42 {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestParseSendsRawTextAndFeedsSensor(t *testing.T) {
	clearEnv(t)
	srv, calls := server(t, func(r recorded) (int, string, http.Header) {
		if r.path == "/api/v1/parse" {
			return 200, `{"ver":"v1","key":42,"dvc":"a+b+","din":[2,0,1]}`, nil
		}
		return 200, `{"status":"success","sensor":"s","index":0}`, nil
	})
	c := newTest(srv.URL, WithAPIKey("k"))
	parsed, err := c.Web.Parse(context.Background(), "var x = 1;")
	if err != nil {
		t.Fatal(err)
	}
	first := (*calls)[0]
	if string(first.body) != "var x = 1;" || !strings.HasPrefix(first.headers.Get("Content-Type"), "text/plain") {
		t.Fatalf("parse request: %s %s", first.headers.Get("Content-Type"), first.body)
	}
	if parsed.Key != 42 || len(parsed.Din) != 3 {
		t.Fatalf("parsed: %+v", parsed)
	}
	if _, err := c.Web.Sensor(context.Background(), WebSensorParams{URL: "https://e.com/", UserAgent: "UA", ScriptData: &parsed.ScriptData}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string((*calls)[1].body), `"scriptData":{"ver":"v1","key":42,"dvc":"a+b+","din":[2,0,1]}`) {
		t.Fatalf("scriptData not forwarded: %s", (*calls)[1].body)
	}
}

func TestSbsdSecCptPixel(t *testing.T) {
	clearEnv(t)
	srv, _ := server(t, func(r recorded) (int, string, http.Header) {
		switch r.path {
		case "/api/v1/sbsd":
			if string(r.body) != `{"vid":"v","bm_o":"o","index":0}` {
				t.Errorf("sbsd body %s", r.body)
			}
			return 200, `{"body":"MTt2","index":0,"vid":"v"}`, nil
		case "/api/v1/sec-cpt":
			if string(r.body) != `{"cookie":"c","difficulty":2000,"count":3}` {
				t.Errorf("sec-cpt body %s", r.body)
			}
			return 200, `{"answers":[1,2,3],"count":3,"attempts":99}`, nil
		default:
			return 200, `{"sensor":"p=1"}`, nil
		}
	})
	c := newTest(srv.URL, WithAPIKey("k"))
	ctx := context.Background()
	if s, err := c.Web.Sbsd(ctx, SbsdParams{Vid: "v", BmO: "o"}); err != nil || s.Body != "MTt2" {
		t.Fatalf("sbsd: %v %v", s, err)
	}
	if s, err := c.Web.SecCpt(ctx, SecCptParams{Cookie: "c", Difficulty: 2000, Count: 3}); err != nil || len(s.Answers) != 3 || s.Attempts != 99 {
		t.Fatalf("sec-cpt: %v %v", s, err)
	}
	if p, err := c.Web.Pixel(ctx, PixelParams{Bazadebezolkohpepadr: "id", Hash: "h"}); err != nil || p.Payload != "p=1" {
		t.Fatalf("pixel: %v %v", p, err)
	}
}

func TestStatusErrorsAreTypedAndNotRetried(t *testing.T) {
	clearEnv(t)
	cases := map[int]error{
		401: ErrUnauthorized,
		402: ErrInsufficientBalance,
		403: ErrPermissionDenied,
		422: ErrUnsupportedTarget,
	}
	for status, sentinel := range cases {
		srv, calls := server(t, func(recorded) (int, string, http.Header) {
			return status, `{"error":"denied","code":"ip_not_whitelisted","request_id":"vr_req1"}`, nil
		})
		c := newTest(srv.URL, WithAPIKey("k"), WithMaxRetries(0))
		_, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "a"})
		var apiErr *APIError
		if !errors.Is(err, sentinel) || !errors.As(err, &apiErr) || apiErr.StatusCode != status || len(*calls) != 1 {
			t.Fatalf("status %d: err=%v calls=%d", status, err, len(*calls))
		}
		if apiErr.Code != "ip_not_whitelisted" || apiErr.RequestID != "vr_req1" {
			t.Fatalf("expected structured error fields, got %+v", apiErr)
		}
	}

	// 408 timeout and 502 bad gateway sentinels
	for status, sentinel := range map[int]error{408: ErrTimeout, 502: ErrBadGateway} {
		srv, _ := server(t, func(recorded) (int, string, http.Header) {
			return status, `{"error":"upstream failed","code":"proxy_connection_failed","retryable":true,"request_id":"vr_gw1"}`, nil
		})
		c := newTest(srv.URL, WithAPIKey("k"), WithMaxRetries(0))
		_, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "a"})
		var apiErr *APIError
		if !errors.Is(err, sentinel) || !errors.As(err, &apiErr) || !apiErr.Retryable {
			t.Fatalf("status %d: err=%v retryable=%v", status, err, apiErr != nil && apiErr.Retryable)
		}
		if status == 502 && !errors.Is(err, ErrServer) {
			t.Fatalf("expected 502 to match ErrServer as well")
		}
	}
}

func TestRetries429ThenSucceeds(t *testing.T) {
	clearEnv(t)
	n := 0
	srv, calls := server(t, func(recorded) (int, string, http.Header) {
		n++
		if n == 1 {
			return 429, `{"error":"request failed"}`, http.Header{"Retry-After": {"1"}}
		}
		return 200, `{"sensor":"ok"}`, nil
	})
	c := newTest(srv.URL, WithAPIKey("k"))
	out, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "a"})
	if err != nil || out.Sensor != "ok" || len(*calls) != 2 {
		t.Fatalf("out=%v err=%v calls=%d", out, err, len(*calls))
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	clearEnv(t)
	srv, calls := server(t, func(recorded) (int, string, http.Header) { return 503, `{"error":"request failed"}`, nil })
	c := newTest(srv.URL, WithAPIKey("k"), WithMaxRetries(2))
	_, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "a"})
	if !errors.Is(err, ErrServiceUnavailable) || !errors.Is(err, ErrServer) || len(*calls) != 3 {
		t.Fatalf("err=%v calls=%d", err, len(*calls))
	}
}

func TestConnectionErrorIsRetriedAndWrapped(t *testing.T) {
	clearEnv(t)
	srv, _ := server(t, func(recorded) (int, string, http.Header) { return 200, `{}`, nil })
	url := srv.URL
	srv.Close()
	c := newTest(url, WithAPIKey("k"), WithMaxRetries(1))
	_, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "a"})
	var ce *ConnectionError
	if !errors.As(err, &ce) {
		t.Fatalf("want ConnectionError, got %v", err)
	}
}

func TestStatusErrorBody(t *testing.T) {
	clearEnv(t)
	srv, _ := server(t, func(recorded) (int, string, http.Header) { return 200, `{"status":"error","err":"x"}`, nil })
	c := newTest(srv.URL, WithAPIKey("k"), WithMaxRetries(0))
	_, err := c.Web.Sensor(context.Background(), WebSensorParams{URL: "https://e.com/", UserAgent: "UA"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
}

func TestMissingKeyFailsBeforeRequest(t *testing.T) {
	clearEnv(t)
	srv, calls := server(t, func(recorded) (int, string, http.Header) { return 200, `{}`, nil })
	c := newTest(srv.URL, WithBMPAPIKey("k_bmp"))
	_, err := c.Web.Sensor(context.Background(), WebSensorParams{URL: "https://e.com/", UserAgent: "UA"})
	if !errors.Is(err, ErrMissingAPIKey) || len(*calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(*calls))
	}
}

func TestEnvKey(t *testing.T) {
	clearEnv(t)
	t.Setenv("VEILROUTE_KEY", "k_env")
	srv, calls := server(t, func(recorded) (int, string, http.Header) { return 200, `{"sensor":"s"}`, nil })
	c := newTest(srv.URL)
	if _, err := c.BMP.Sensor(context.Background(), BMPSensorParams{App: "a"}); err != nil {
		t.Fatal(err)
	}
	if (*calls)[0].headers.Get("X-Api-Key") != "k_env" {
		t.Fatal("env key not used")
	}
}

func TestStringNeverContainsKey(t *testing.T) {
	c := NewClient(WithAPIKey("super_secret_key"))
	if strings.Contains(c.String(), "super_secret_key") {
		t.Fatal("key leaked")
	}
}

func TestAbckIsValid(t *testing.T) {
	if !AbckIsValid("ABC~0~-1~-1") || AbckIsValid("ABC~-1~-1~-1") || AbckIsValid("") {
		t.Fatal("AbckIsValid")
	}
}
