<p align="center">
  <a href="https://veilroute.dev"><img src="https://veilroute.dev/favicon.svg" width="64" height="64" alt="VeilRoute"></a>
</p>

<h1 align="center">VeilRoute Go SDK</h1>

<p align="center">
  Akamai Bot Manager sensor API for Go: <code>sensor_data</code>, <code>sbsd</code>, <code>sec-cpt</code>, pixel and BMP mobile sensors.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/veilroutesdk/veilroute-go"><img src="https://pkg.go.dev/badge/github.com/veilroutesdk/veilroute-go.svg" alt="Go Reference"></a>
  <a href="https://github.com/veilroutesdk/veilroute-go/actions/workflows/ci.yml"><img src="https://github.com/veilroutesdk/veilroute-go/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/dependencies-0-brightgreen.svg" alt="Zero dependencies">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT"></a>
</p>

<p align="center">
  <a href="https://veilroute.dev">Website</a> ·
  <a href="https://docs.veilroute.dev">Docs</a> ·
  <a href="https://app.veilroute.dev">Get an API key</a> ·
  <a href="https://discord.gg/YrZAcNWtE">Discord</a>
</p>

---

[VeilRoute](https://veilroute.dev) generates request-ready **Akamai Bot Manager** payloads over HTTP, so you can
get a trusted `_abck` cookie without running a headless browser. This module is the official Go client.

- **Akamai Web (v3):** staged `sensor_data` (index 0, 1, 2), `sbsd`, `sec-cpt` proof of work, pixel, script parsing.
- **Akamai BMP:** Mobile sensor API for the `x-acf-sensor-data` header and session cookies.
- Standard library only, `context.Context` everywhere, `errors.Is` sentinels, automatic retries on `503` and network errors.
- You are only billed for sensors that return `200`, so retries are free.

## Install

```bash
go get github.com/veilroutesdk/veilroute-go
```

Requires Go 1.21+.

## Quickstart

Get a key from the [dashboard](https://app.veilroute.dev) and export it:

```bash
export VEILROUTE_KEY="your-api-key"
```

### Akamai Web: generate sensor

```go
import "github.com/veilroutesdk/veilroute-go"

client := veilroute.NewClient() // reads VEILROUTE_KEY

s, err := client.Web.Sensor(ctx, veilroute.WebSensorParams{
	URL:       "https://www.example.com/",
	UserAgent: ua,                        // the exact UA your HTTP client sends
	Cookies:   "_abck=...; bm_sz=...",    // cookies from your first GET
	Index:     0,
	SessionID: "sess_42",
})
if err != nil {
	return err
}

if veilroute.AbckIsValid(newAbck) {
	// valid
}
```

Repeat with `Index: 1` and `Index: 2` (same `SessionID`) if the site needs more posts.

### Akamai BMP: mobile sensor

```go
bmp, err := client.BMP.Sensor(ctx, veilroute.BMPSensorParams{
	App:      "smiles", // or "azul"
	Proxy:    "http://user:pass@host:port",
	Language: "pt-BR",
})
if err != nil {
	return err
}

// Set the sensor header
req.Header.Set(veilroute.HeaderName, bmp.Sensor) // x-acf-sensor-data

// Set returned cookies, if present
if cookieHeader := bmp.CookieHeader(); cookieHeader != "" {
	req.Header.Set("Cookie", cookieHeader)
}
```

Send your request with an OkHttp Android TLS fingerprint
(for example [`bogdanfinn/tls-client`](https://github.com/bogdanfinn/tls-client) with an OkHttp profile).

## All methods

| Method | Endpoint | Returns |
| --- | --- | --- |
| `client.Web.Sensor(ctx, WebSensorParams)` | `POST web/api/v1/sensor` | `*WebSensor{Sensor, Index, SessionID, Ms}` |
| `client.Web.Sbsd(ctx, SbsdParams)` | `POST web/api/v1/sbsd` | `*Sbsd{Body, Index, Vid}` |
| `client.Web.SecCpt(ctx, SecCptParams)` | `POST web/api/v1/sec-cpt` | `*SecCpt{Answers, Count, Attempts}` |
| `client.Web.Pixel(ctx, PixelParams)` | `POST web/api/v1/pixel` | `*Pixel{Payload}` |
| `client.Web.Parse(ctx, script)` | `POST web/api/v1/parse` | `*ParsedScript{Ver, Key, Dvc, Din}` |
| `client.BMP.Sensor(ctx, BMPSensorParams)` | `POST bmp/api/v1/sensor` | `*BMPSensor{Sensor, DeviceID, Cookies, SDKVersion, ...}` |

Every result keeps the full JSON in `.Raw`. Field-by-field reference: [docs.veilroute.dev](https://docs.veilroute.dev).

## Configuration

```go
client := veilroute.NewClient(
	veilroute.WithAPIKey("..."),        // or VEILROUTE_KEY
	veilroute.WithWebAPIKey("..."),     // optional, per product (or VEILROUTE_WEB_KEY)
	veilroute.WithBMPAPIKey("..."),     // optional, per product (or VEILROUTE_BMP_KEY)
	veilroute.WithTimeout(30*time.Second),
	veilroute.WithMaxRetries(2),        // 502, 503, 504 and network errors, exponential backoff
	veilroute.WithHTTPClient(myClient), // optional
)
```

Keys are **per product**: an Akamai Web key does not work on BMP. Keep keys server-side.

## Errors

```go
s, err := client.Web.Sensor(ctx, params)
switch {
case errors.Is(err, veilroute.ErrInsufficientBalance): // 402: top up at app.veilroute.dev
case err != nil:
	var apiErr *veilroute.APIError
	if errors.As(err, &apiErr) {
		log.Println(apiErr.StatusCode)
	}
}
```

| Sentinel | Status |
| --- | --- |
| `ErrBadRequest` | 400 |
| `ErrUnauthorized` | 401 |
| `ErrInsufficientBalance` | 402 |
| `ErrNotFound` | 404 |
| `ErrUnsupportedTarget` | 422 |
| `ErrServiceUnavailable` / `ErrServer` | 503 / 5xx |
| `*ConnectionError` | network, timeout, cancelled context |
| `ErrMissingAPIKey` | no key configured |

## Your TLS fingerprint still matters

The sensor is half of the story. Akamai also reads your TLS ClientHello and header order, so send your own
request with a client that impersonates **Chrome** (Web) or **OkHttp** (BMP), such as
[`bogdanfinn/tls-client`](https://github.com/bogdanfinn/tls-client). See
[HTTP client & TLS](https://docs.veilroute.dev/http-client).

## Other SDKs

- Python: [veilroutesdk/veilroute-python](https://github.com/veilroutesdk/veilroute-python)
- Node.js / TypeScript: [veilroutesdk/veilroute-node](https://github.com/veilroutesdk/veilroute-node)

## License

[MIT](LICENSE)
