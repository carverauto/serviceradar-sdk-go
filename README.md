# serviceradar-sdk-go

ServiceRadar plugin SDK for Go (TinyGo/WASM).

## Overview
This SDK lets you write ServiceRadar plugin checkers in Go without handling low-level WASM host calls. It handles:
- Config decoding from the host
- Result builder for `serviceradar.plugin_result.v1`
- Logging bridge
- HTTP/TCP/UDP proxy wrappers
- Support for Websockets
- Device discovery envelopes for inventory-producing plugins
- Event emission + alert promotion hints
- Signal schema/display contract references for package-managed logs and events

## Install

```bash
go get code.carverauto.dev/carverauto/serviceradar-sdk-go
```

## Example

```go
package main

import (
    "context"
    "fmt"
    "code.carverauto.dev/carverauto/serviceradar-sdk-go/sdk"
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

### Signal display contracts
When a plugin emits OCSF events or OTEL-style logs that are described by a package manifest, attach the package schema/display reference through the SDK:

```go
event := sdk.NewOCSFEventLogActivity("camera motion", sdk.SeverityWarning)
sdk.AttachSignalSchemaRef(&event, sdk.SignalSchemaRef{
    ProducerID:             "axis-camera",
    ProducerVersion:        "0.1.0",
    SchemaID:               "com.carverauto.axis_camera.event_log",
    SchemaVersion:          "1.0.0",
    DisplayContractID:      "com.carverauto.axis_camera.event_log.display",
    DisplayContractVersion: "1.0.0",
    DisplayContract:        "display/event_log_activity.display.json",
    SignalType:             sdk.SignalSchemaSignalTypeEvent,
    PayloadKind:            sdk.SignalSchemaPayloadKindOCSFEvent,
})
```

The helper writes the ServiceRadar extension metadata under `metadata.service_radar.signal_schema`.

For first-class telemetry that should be ingested independently of the check result,
declare the `emit_telemetry` capability and send a telemetry batch:

```go
event := sdk.NewOCSFEventLogActivity("camera motion", sdk.SeverityWarning)
record := sdk.NewOCSFTelemetryRecord(event).WithSignalSchemaRef(sdk.SignalSchemaRef{
    ProducerID:             "axis-camera",
    ProducerVersion:        "0.1.0",
    SchemaID:               "com.carverauto.axis_camera.event_log",
    SchemaVersion:          "1.0.0",
    DisplayContractID:      "com.carverauto.axis_camera.event_log.display",
    DisplayContractVersion: "1.0.0",
    DisplayContract:        "display/event_log_activity.display.json",
    SignalType:             sdk.SignalSchemaSignalTypeEvent,
    PayloadKind:            sdk.SignalSchemaPayloadKindOCSFEvent,
})

err := sdk.EmitTelemetry(sdk.TelemetryBatch{
    Source: sdk.TelemetrySource{
        SourceType:     "axis-camera",
        SourceInstance: "front-door",
    },
    Records: []sdk.TelemetryRecord{record},
})
```

Use result-attached `events` for check-scoped annotations. Use `EmitTelemetry` for
standalone or streaming plugin logs/events.

### Context-aware I/O
Context variants exist for host I/O to match Go expectations:
- HTTP: `HTTP.DoContext`, `HTTP.GetContext`, `HTTP.PostContext`
- TCP: `TCPDialContext`, `(*TCPConn).ReadContext`, `(*TCPConn).WriteContext`
- UDP: `UDPSendToContext`
- WebSocket: `WebSocketDialContext`, `(*WebSocketConn).SendContext`, `(*WebSocketConn).RecvContext`

These currently check `ctx.Err()` before the host call (TinyGo/Wasm is synchronous), but give you a stable API if cancellation support is added later.

### WebSocket Support
The SDK provides WebSocket client capabilities for plugins that need to communicate with WebSocket servers:

```go
// Dial a WebSocket endpoint
conn, err := sdk.WebSocketDialContext(ctx, "ws://localhost:8080/ws", 10*time.Second)
if err != nil {
    return nil, fmt.Errorf("websocket dial failed: %w", err)
}
defer conn.Close()

// Send a message
if err := conn.SendContext(ctx, []byte(`{"method": "getInfo"}`), 10*time.Second); err != nil {
    return nil, fmt.Errorf("websocket send failed: %w", err)
}

// Read response
buf := make([]byte, 4096)
n, err := conn.RecvContext(ctx, buf, 10*time.Second)
if err != nil {
    return nil, fmt.Errorf("websocket recv failed: %w", err)
}
data := buf[:n]
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

### Source-native local development

Non-TinyGo builds can install a local development host and run the same SDK
config, HTTP, logging, telemetry, and result calls used by the Wasm build. No
Wasm artifact, signature, registry, or cluster is required.

`LoadLocalInputs` accepts public config and optional action-invocation JSON from
files or environment variables. It also reads an optional `.env` file. Process
environment values override `.env` values. Credential fields use the
`SERVICERADAR_CREDENTIAL_` prefix and remain separate from runtime config:

```dotenv
SERVICERADAR_PLUGIN_CONFIG_FILE=testdata/config.json
SERVICERADAR_PLUGIN_ACTION_FILE=testdata/action.json
SERVICERADAR_CREDENTIAL_USERNAME=local-user
SERVICERADAR_CREDENTIAL_PASSWORD=local-password
```

```go
inputs, err := sdk.LoadLocalInputs(sdk.LocalInputOptions{})
if err != nil {
    return err
}
runtimeConfig, err := inputs.RuntimeConfigJSON()
if err != nil {
    return err
}

credentials := inputs.Credentials()
capture, err := sdk.RunLocalHost(sdk.LocalHostOptions{
    ConfigJSON: runtimeConfig,
    HTTPHandler: newLocalBroker(credentials), // host-owned auth and endpoint policy
}, runPlugin)
```

The HTTP callback is the trusted local host adapter. It should enforce the same
exact endpoint grant, credential injection, redirects, TLS, and response bounds
as production. Do not add credentials to plugin config, action input, logs, or
results. A successful local run does not bypass production package admission.

### Policy input payload helpers (`serviceradar.plugin_inputs.v1`)
For policy-driven plugin assignments, decode and validate the typed input payload:

```go
var payload sdk.PluginInputsPayload

if err := sdk.LoadConfig(&payload); err != nil {
    return nil, err
}
if err := payload.Validate(); err != nil {
    return nil, err
}

// Iterate all resolved items (devices/interfaces/etc.)
err := payload.EachItem(func(item sdk.PluginInputItem) error {
    // item.Entity: "devices" | "interfaces" | ...
    // item.Item:   map with resolved fields (uid/ip/if_name/etc.)
    return nil
})
if err != nil {
    return nil, err
}

devices := payload.ItemsByEntity("devices")
_ = devices
```

Helpers also include:
- `sdk.ParsePluginInputsJSON([]byte)`
- `sdk.ParsePluginInputsMap(map[string]any)`
- `(*PluginInputsPayload).FlattenItems()`
- `(*PluginInputsPayload).ItemsByEntity(string)`
- `(*PluginInputsPayload).ItemsByName(string)`

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
- `emit_telemetry`
- `http_request`
- `tcp_connect` / `tcp_read` / `tcp_write` / `tcp_close`
- `udp_sendto`
- `websocket_connect` / `websocket_send` / `websocket_recv` / `websocket_close`
- `camera_media_open` / `camera_media_write` / `camera_media_heartbeat` / `camera_media_close`

The SDK wraps these functions and exports `alloc`/`dealloc` for host memory access.

## Event and Alert Hints

The SDK emits optional fields in the result payload to support event promotion:
- `events`: list of OCSF Event Log Activity objects
- `alert_hint`: boolean flag for immediate promotion
- `condition_id`: string used for de-duplication and auto-clear logic

These fields are ignored safely by older control-plane builds.
