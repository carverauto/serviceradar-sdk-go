# serviceradar-sdk-go

ServiceRadar plugin SDK for Go (TinyGo/WASM).

## Overview
This SDK lets you write ServiceRadar plugin checkers in Go without handling low-level WASM host calls. It handles:
- Config decoding from the host
- Result builder for `serviceradar.plugin_result.v1`
- Logging bridge
- HTTP/TCP/UDP proxy wrappers
- Support for Websockets
- Event emission + alert promotion hints

## Install

```bash
go get github.com/carverauto/serviceradar-sdk-go
```

## Example

```go
package main

import (
    "context"
    "fmt"
    "github.com/carverauto/serviceradar-sdk-go/sdk"
)

type Config struct {
    URL     string  `json:"url"`
    WarnMS  float64 `json:"warn_ms"`
    CritMS  float64 `json:"crit_ms"`
}

//export run_check
func run_check() {
    _ = sdk.Execute(func() (*sdk.Result, error) {
        var cfg Config

        if err := sdk.LoadConfig(&cfg); err != nil {
            return nil, err
        }

        resp, err := sdk.HTTP.GetContext(context.Background(), cfg.URL)
        if err != nil {
            return nil, fmt.Errorf("http request failed: %w", err)
        }

        latency := float64(resp.Duration.Milliseconds())
        thresholds := sdk.Thresholds(cfg.WarnMS, cfg.CritMS)

        return sdk.NewResult().
            WithSummary(fmt.Sprintf("http %d in %.0fms", resp.Status, latency)).
            WithThresholds(latency, thresholds.Warn, thresholds.Crit).
            WithMetric("latency_ms", latency, "ms", thresholds).
            WithStatCard("Latency", fmt.Sprintf("%.0fms", latency), "success"), nil
    })
}

func main() {}

```

## Examples

- `examples/http-check`: HTTP latency check with thresholds and events
- `examples/tcp-check`: TCP connectivity check with optional write/read
- `examples/udp-check`: UDP send check with bytes-sent metric
- `examples/widgets-check`: HTTP check demonstrating stat card, table, sparkline, and markdown widgets

## API ergonomics

### Execute and error handling
`Execute` accepts a function that returns `(*Result, error)` and itself returns an `error`:

```go
err := sdk.Execute(func() (*sdk.Result, error) {
    // ...
    return sdk.Ok("ok"), nil
})
if err != nil {
    // Optional: handle submit/serialize errors (logging is already done by the SDK)
}
```

If your function returns a non-nil error, the SDK auto-generates a critical result (or upgrades your result to critical) and records the error details in the payload. This keeps the happy path concise while still surfacing failures.

### Defaults and zero-value behavior
Defaults are applied at the edge (right before serialization) so `Serialize` does not mutate the original object:
- `SchemaVersion` defaults to `1`
- `Status` defaults to `UNKNOWN`
- `Summary` defaults to the status string
- `ObservedAt` defaults to “now” in RFC3339Nano

This means `var r sdk.Result` is safe; serialization produces a valid payload without altering `r`.

### Fluent builders
Result has both conventional setters (`SetSummary`, `AddMetric`, etc.) and fluent builders (`WithSummary`, `WithMetric`, etc.) so you can choose style:

```go
return sdk.NewResult().
    WithSummary("all good").
    WithMetric("cpu", 10, "%", nil).
    WithLabel("version", "1.2.3"), nil
```

### Threshold helpers
Use `ThresholdSpec` for warning/critical thresholds and `Thresholds(warn, crit)` to build one without helper functions:

```go
thresholds := sdk.Thresholds(50, 100)
res.WithMetric("latency_ms", 10, "ms", thresholds)
res.WithThresholds(10, thresholds.Warn, thresholds.Crit)
```

### Context-aware I/O
Context variants exist for host I/O to match Go expectations:
- HTTP: `HTTP.DoContext`, `HTTP.GetContext`, `HTTP.PostContext`
- TCP: `TCPDialContext`, `(*TCPConn).ReadContext`, `(*TCPConn).WriteContext`
- UDP: `UDPSendToContext`
- WebSocket: `WebSocketDialContext`, `(*WebSocketConn).ReadContext`, `(*WebSocketConn).WriteContext`

These currently check `ctx.Err()` before the host call (TinyGo/Wasm is synchronous), but give you a stable API if cancellation support is added later.

### WebSocket Support
The SDK provides WebSocket client capabilities for plugins that need to communicate with WebSocket servers:

```go
// Dial a WebSocket endpoint
conn, err := sdk.WebSocketDialContext(ctx, "ws://localhost:8080/ws")
if err != nil {
    return nil, fmt.Errorf("websocket dial failed: %w", err)
}
defer conn.Close()

// Send a message
if err := conn.WriteContext(ctx, []byte(`{"method": "getInfo"}`)); err != nil {
    return nil, fmt.Errorf("websocket write failed: %w", err)
}

// Read response
data, err := conn.ReadContext(ctx)
if err != nil {
    return nil, fmt.Errorf("websocket read failed: %w", err)
}
```

WebSocket connections are mediated by the host runtime, which enforces:
- **Domain allowlists**: Only permitted domains can be connected to
- **Port restrictions**: Only allowed ports can be accessed
- **Connection limits**: Maximum concurrent connections per plugin

The plugin must have the following capabilities in its manifest:
- `websocket_connect`: Permission to establish WebSocket connections
- `websocket_send`: Permission to send messages
- `websocket_recv`: Permission to receive messages
- `websocket_close`: Permission to close connections

To include headers (for example Authorization) on the initial WebSocket handshake:

```go
headers := map[string]string{
  "Authorization": "Basic <base64-user-pass>",
}
conn, err := sdk.WebSocketConnectWithHeaders("wss://camera.local/vapix/ws-data-stream?sources=events", headers, 10*time.Second)
```

### Config loading
`LoadConfig` is an alias of `GetConfig` for more idiomatic naming in user code:

```go
if err := sdk.LoadConfig(&cfg); err != nil {
    return nil, err
}
```

## Build

```bash
# Requires TinyGo
cd examples/http-check

tinygo build -o plugin.wasm -target=wasi ./
```

## Host ABI
The agent imports host functions from the `env` module:
- `get_config`
- `log`
- `submit_result`
- `http_request`
- `tcp_connect` / `tcp_read` / `tcp_write` / `tcp_close`
- `udp_sendto`
- `websocket_connect` / `websocket_send` / `websocket_recv` / `websocket_close`

The SDK wraps these functions and exports `alloc`/`dealloc` for host memory access.

## Event and Alert Hints

The SDK emits optional fields in the result payload to support event promotion:
- `events`: list of OCSF Event Log Activity objects
- `alert_hint`: boolean flag for immediate promotion
- `condition_id`: string used for de-duplication and auto-clear logic

These fields are ignored safely by older control-plane builds.
