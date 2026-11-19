// Package veilroute is the official Go client for VeilRoute, the Akamai Bot
// Manager sensor API: Akamai Web sensor_data, sbsd, sec-cpt, pixel and script
// parsing, plus Akamai BMP mobile sensors.
//
//	client := veilroute.NewClient() // reads VEILROUTE_KEY
//	s, err := client.Web.Sensor(ctx, veilroute.WebSensorParams{
//		URL:       "https://www.example.com/",
//		UserAgent: ua,
//		Cookies:   "_abck=...; bm_sz=...",
//		SessionID: "sess_42",
//	})
//
// Docs: https://docs.veilroute.dev
package veilroute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version of this SDK.
const Version = "0.1.0"

// Default API hosts.
const (
	DefaultWebURL = "https://web.veilroute.dev"
	DefaultBMPURL = "https://bmp.veilroute.dev"
)

const (
	productWeb = "web"
	productBMP = "bmp"

	maxRetryAfter = 30 * time.Second
	maxBodyBytes  = 4 << 20
)

// Client talks to the VeilRoute API. It is safe for concurrent use.
type Client struct {
	Web *WebService
	BMP *BMPService

	keys       map[string]string
	urls       map[string]string
	httpClient *http.Client
	maxRetries int
	headers    http.Header
	sleep      func(context.Context, time.Duration) error
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the key used for both products.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.keys[productWeb], c.keys[productBMP] = key, key }
}

// WithWebAPIKey sets the Akamai Web key.
func WithWebAPIKey(key string) Option { return func(c *Client) { c.keys[productWeb] = key } }

// WithBMPAPIKey sets the Akamai BMP key.
func WithBMPAPIKey(key string) Option { return func(c *Client) { c.keys[productBMP] = key } }

// WithHTTPClient replaces the default http.Client (30s timeout).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithTimeout sets the per-attempt timeout of the default http.Client.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient = &http.Client{Timeout: d} }
}

// WithMaxRetries sets how many times 429/502/503/504 and network errors are
// retried (default 2). Only 200 responses are billed, so retrying is safe.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithWebBaseURL overrides the Akamai Web host.
func WithWebBaseURL(u string) Option {
	return func(c *Client) { c.urls[productWeb] = strings.TrimRight(u, "/") }
}

// WithBMPBaseURL overrides the Akamai BMP host.
func WithBMPBaseURL(u string) Option {
	return func(c *Client) { c.urls[productBMP] = strings.TrimRight(u, "/") }
}

// WithHeader adds a header to every request.
func WithHeader(key, value string) Option { return func(c *Client) { c.headers.Set(key, value) } }

func firstEnv(names ...string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}

func orDefault(v, d string) string {
	if v != "" {
		return v
	}
	return d
}

// NewClient builds a client. Keys default to VEILROUTE_KEY, VEILROUTE_WEB_KEY
// and VEILROUTE_BMP_KEY; options override them.
func NewClient(opts ...Option) *Client {
	shared := firstEnv("VEILROUTE_KEY", "VEILROUTE_API_KEY")
	c := &Client{
		keys: map[string]string{
			productWeb: orDefault(os.Getenv("VEILROUTE_WEB_KEY"), shared),
			productBMP: orDefault(os.Getenv("VEILROUTE_BMP_KEY"), shared),
		},
		urls: map[string]string{
			productWeb: strings.TrimRight(orDefault(os.Getenv("VEILROUTE_WEB_URL"), DefaultWebURL), "/"),
			productBMP: strings.TrimRight(orDefault(os.Getenv("VEILROUTE_BMP_URL"), DefaultBMPURL), "/"),
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
		maxRetries: 2,
		headers:    http.Header{},
		sleep:      sleepCtx,
	}
	for _, o := range opts {
		o(c)
	}
	c.Web = &WebService{c: c}
	c.BMP = &BMPService{c: c}
	return c
}

// String never includes the keys.
func (c *Client) String() string {
	var products []string
	for _, p := range []string{productWeb, productBMP} {
		if c.keys[p] != "" {
			products = append(products, p)
		}
	}
	return fmt.Sprintf("veilroute.Client{products: %v, maxRetries: %d}", products, c.maxRetries)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func backoff(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if s, err := strconv.ParseFloat(retryAfter, 64); err == nil && s >= 0 {
			d := time.Duration(s * float64(time.Second))
			if d > maxRetryAfter {
				d = maxRetryAfter
			}
			return d
		}
	}
	base := 500 * time.Millisecond << attempt
	if base > 8*time.Second {
		base = 8 * time.Second
	}
	return time.Duration(float64(base) * (0.75 + rand.Float64()*0.5))
}

func retryable(status int) bool {
	return status == 429 || status == 502 || status == 503 || status == 504
}

// post sends body (JSON-encoded, or raw text when it is a string) and decodes
// the JSON response into out.
func (c *Client) post(ctx context.Context, product, path string, body any, out any) error {
	key := c.keys[product]
	if key == "" {
		return fmt.Errorf("%w: no key for Akamai %s; use WithAPIKey or set VEILROUTE_KEY (or VEILROUTE_%s_KEY)",
			ErrMissingAPIKey, strings.ToUpper(product), strings.ToUpper(product))
	}
	var payload []byte
	contentType := "application/json"
	if s, ok := body.(string); ok {
		payload, contentType = []byte(s), "text/plain; charset=utf-8"
	} else {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("veilroute: encode request: %w", err)
		}
	}
	url := c.urls[product] + path

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("veilroute: build request: %w", err)
		}
		for k, v := range c.headers {
			req.Header[k] = v
		}
		req.Header.Set("Accept", "application/json")
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "veilroute-go/"+Version)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Api-Key", key)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return &ConnectionError{Err: ctx.Err()}
			}
			if attempt < c.maxRetries {
				if serr := c.sleep(ctx, backoff(attempt, "")); serr != nil {
					return &ConnectionError{Err: serr}
				}
				continue
			}
			return &ConnectionError{Err: err}
		}
		raw, rerr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
		resp.Body.Close()
		if rerr != nil {
			return &ConnectionError{Err: rerr}
		}
		if retryable(resp.StatusCode) && attempt < c.maxRetries {
			if serr := c.sleep(ctx, backoff(attempt, resp.Header.Get("Retry-After"))); serr != nil {
				return &ConnectionError{Err: serr}
			}
			continue
		}
		return decode(resp.StatusCode, raw, out)
	}
}

func decode(status int, raw []byte, out any) error {
	var generic map[string]any
	_ = json.Unmarshal(raw, &generic)
	if status >= 400 {
		return newAPIError(status, generic)
	}
	if generic == nil {
		return &APIError{StatusCode: status, Message: "invalid response body"}
	}
	if s, _ := generic["status"].(string); s == "error" {
		return newAPIError(503, generic)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &APIError{StatusCode: status, Message: "invalid response body"}
	}
	if r, ok := out.(interface{ setRaw(map[string]any) }); ok {
		r.setRaw(generic)
	}
	return nil
}

// AbckIsValid reports whether an _abck cookie value is valid.
func AbckIsValid(abck string) bool { return strings.Contains(abck, "~0~") }
