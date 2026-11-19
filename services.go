package veilroute

import (
	"context"
	"strings"
)

type rawHolder struct {
	// Raw is the full JSON response.
	Raw map[string]any `json:"-"`
}

func (r *rawHolder) setRaw(m map[string]any) { r.Raw = m }

// ScriptData holds Akamai script constants (see WebService.Parse).
type ScriptData struct {
	Ver string `json:"ver,omitempty"`
	Key int64  `json:"key,omitempty"`
	Dvc string `json:"dvc,omitempty"`
	Din []int  `json:"din,omitempty"`
}

// ---------------------------------------------------------------- Akamai Web

// WebService groups the Akamai Web (Bot Manager v3) endpoints on web.veilroute.dev.
type WebService struct{ c *Client }

// WebSensorParams is the body of POST /api/v1/sensor.
type WebSensorParams struct {
	URL        string      `json:"url"`
	UserAgent  string      `json:"userAgent"`
	Index      int         `json:"index"`
	SessionID  string      `json:"session_id,omitempty"`
	Cookies    string      `json:"cookies,omitempty"`
	Abck       string      `json:"_abck,omitempty"`
	BmSz       string      `json:"bm_sz,omitempty"`
	ScriptData *ScriptData `json:"scriptData,omitempty"`
	ScriptURL  string      `json:"scriptUrl,omitempty"`
	Language   string      `json:"language,omitempty"`
}

// WebSensor is a generated sensor. Post Sensor as the body of the site's sensor script path.
type WebSensor struct {
	Sensor    string `json:"sensor"`
	Index     int    `json:"index"`
	SessionID string `json:"session_id"`
	Ms        int    `json:"ms"`
	rawHolder
}

// Sensor generates the sensor payload for step Index (0, 1, 2).
func (s *WebService) Sensor(ctx context.Context, p WebSensorParams) (*WebSensor, error) {
	out := &WebSensor{}
	if err := s.c.post(ctx, productWeb, "/api/v1/sensor", p, out); err != nil {
		return nil, err
	}
	return out, nil
}

// SbsdParams is the body of POST /api/v1/sbsd.
type SbsdParams struct {
	Vid       string `json:"vid"`
	BmO       string `json:"bm_o"`
	T         string `json:"t,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	URL       string `json:"url,omitempty"`
	Language  string `json:"language,omitempty"`
	Index     int    `json:"index"`
}

// Sbsd holds the base64 body to post to the site's sbsd endpoint.
type Sbsd struct {
	Body  string `json:"body"`
	Index int    `json:"index"`
	Vid   string `json:"vid"`
	rawHolder
}

// Sbsd builds the sbsd payload.
func (s *WebService) Sbsd(ctx context.Context, p SbsdParams) (*Sbsd, error) {
	out := &Sbsd{}
	if err := s.c.post(ctx, productWeb, "/api/v1/sbsd", p, out); err != nil {
		return nil, err
	}
	return out, nil
}

// SecCptParams is the body of POST /api/v1/sec-cpt.
type SecCptParams struct {
	Cookie     string `json:"cookie"`
	Nonce      string `json:"nonce,omitempty"`
	Difficulty int    `json:"difficulty,omitempty"`
	Count      int    `json:"count,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

// SecCpt holds the proof-of-work answers, in order.
type SecCpt struct {
	Answers  []int64 `json:"answers"`
	Count    int     `json:"count"`
	Attempts int64   `json:"attempts"`
	rawHolder
}

// SecCpt solves a sec-cpt proof-of-work challenge.
func (s *WebService) SecCpt(ctx context.Context, p SecCptParams) (*SecCpt, error) {
	out := &SecCpt{}
	if err := s.c.post(ctx, productWeb, "/api/v1/sec-cpt", p, out); err != nil {
		return nil, err
	}
	return out, nil
}

// PixelParams is the body of POST /api/v1/pixel.
type PixelParams struct {
	Bazadebezolkohpepadr string `json:"bazadebezolkohpepadr"`
	Hash                 string `json:"hash"`
	UserAgent            string `json:"userAgent,omitempty"`
}

// Pixel holds the URL-encoded payload to post to the site's pixel endpoint.
type Pixel struct {
	Payload string `json:"payload"`
	rawHolder
}

// Pixel builds the pixel validation payload.
func (s *WebService) Pixel(ctx context.Context, p PixelParams) (*Pixel, error) {
	out := &Pixel{}
	if err := s.c.post(ctx, productWeb, "/api/v1/pixel", p, out); err != nil {
		return nil, err
	}
	if out.Payload == "" {
		out.Payload, _ = out.Raw["sensor"].(string)
	}
	return out, nil
}

// ParsedScript is the result of WebService.Parse.
type ParsedScript struct {
	ScriptData
	rawHolder
}

// Parse extracts ver, key, dvc and din from a script payload. Pass
// &parsed.ScriptData as WebSensorParams.ScriptData.
func (s *WebService) Parse(ctx context.Context, script string) (*ParsedScript, error) {
	out := &ParsedScript{}
	if err := s.c.post(ctx, productWeb, "/api/v1/parse", script, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------- Akamai BMP

// Cookie represents an HTTP cookie returned with the sensor response.
type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain,omitempty"`
}

// BMPService groups the Akamai BMP (mobile SDK) endpoints on bmp.veilroute.dev.
type BMPService struct{ c *Client }

// BMPSensorParams is the body of POST /api/v1/sensor on the BMP host.
type BMPSensorParams struct {
	// App is the target application name (e.g. "smiles", "azul") or package identifier.
	App string `json:"app"`
	// Proxy is an optional proxy URL (e.g. "http://user:pass@host:port").
	Proxy string `json:"proxy,omitempty"`
	// Language is the device language tag (e.g. "pt-BR").
	Language string `json:"language,omitempty"`
	// SessionID is an optional session identifier.
	SessionID string `json:"sessionId,omitempty"`
	// DeviceID is an optional stable device UUID to keep fingerprint consistency across calls.
	DeviceID string `json:"deviceId,omitempty"`
}

// BMPSensor is a mobile sensor. Send Sensor as the x-acf-sensor-data header.
type BMPSensor struct {
	Sensor             string   `json:"sensor"`
	DeviceID           string   `json:"deviceId"`
	Platform           string   `json:"platform"`
	Android            string   `json:"android"`
	SDKVersion         string   `json:"sdkVersion"`
	AppVersion         string   `json:"appVersion,omitempty"`
	AppVersionCode     int      `json:"appVersionCode,omitempty"`
	DeviceModel        string   `json:"deviceModel,omitempty"`
	DeviceManufacturer string   `json:"deviceManufacturer,omitempty"`
	ScreenWidth        int      `json:"screenWidth,omitempty"`
	ScreenHeight       int      `json:"screenHeight,omitempty"`
	Language           string   `json:"language,omitempty"`
	SessionID          string   `json:"sessionId,omitempty"`
	LatencyMs          int64    `json:"latencyMs"`
	Cookies            []Cookie `json:"cookies,omitempty"`
	rawHolder
}

// CookieHeader formats the returned cookies as a standard "Cookie: ..." header value.
func (b *BMPSensor) CookieHeader() string {
	if len(b.Cookies) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, c := range b.Cookies {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(c.Name)
		sb.WriteString("=")
		sb.WriteString(c.Value)
	}
	return sb.String()
}

// HeaderName is the header that carries the BMP sensor.
const HeaderName = "x-acf-sensor-data"

// Sensor generates a mobile sensor for the x-acf-sensor-data header.
func (s *BMPService) Sensor(ctx context.Context, p BMPSensorParams) (*BMPSensor, error) {
	out := &BMPSensor{}
	if err := s.c.post(ctx, productBMP, "/api/v1/sensor", p, out); err != nil {
		return nil, err
	}
	return out, nil
}
