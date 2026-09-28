# serviceradar-sdk-go

ServiceRadar plugin SDK for Go (TinyGo/WASM).

Source of truth: https://github.com/carverauto/serviceradar-sdk-go

## Overview
This SDK lets you write ServiceRadar plugin checkers in Go without handling low-level WASM host calls. It handles:
- Config decoding from the host
- Result builder for `serviceradar.plugin_result.v1`
- Logging bridge
- HTTP/TCP/UDP proxy wrappers
- Support for Websockets
- Device discovery envelopes for inventory-producing plugins
- Event emission + alert promotion hints
- First-class metric telemetry helper for canonical `serviceradar.metric.v1` payloads
- Signal schema/display contract references for package-managed logs and events
- Advisory-feed contract builders and gateway-mediated artifact staging helpers
- Run overrides for time-bounded plugin state, and a one-call OCSF event emitter
- Host-proxied unary gRPC calls
- HTTP response headers (envelope mode) with `Retry-After` parsing
- Typed credential broker grants, including OAuth2 client-credentials injection

## Install

```bash
go get github.com/carverauto/serviceradar-sdk-go/v2@latest
```

### Fetching the module

The public Go module proxy and checksum database do not serve these modules,
so tell the Go toolchain to fetch them directly from GitHub:

```bash
export GOPRIVATE=github.com/carverauto/*
export GONOSUMDB=github.com/carverauto/*
```

## Example

```go
package main

import (
    "context"
    "fmt"
    "github.com/carverauto/serviceradar-sdk-go/v2/sdk"
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
- `examples/sample-northbound`: northbound action plugin (device lookup, interface audit) with no external dependency

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
Result has both conventional setters (`SetSummary`, `AddLabel`, etc.) and fluent builders (`WithSummary`, `WithLabel`, etc.) so you can choose style:

```go
return sdk.NewResult().
    WithSummary("all good").
    WithLabel("version", "1.2.3"), nil
```

### Metric telemetry
Do not put time-series metrics in `serviceradar.plugin_result.v1`. Result
metrics are no longer serialized by the SDK and are rejected by current
ServiceRadar agents. Emit canonical metric protobuf batches with
`emit_telemetry` instead:

```go
record := sdk.NewServiceRadarMetricTelemetryRecordFromBatch("metric-event-1", sdk.MetricBatch{
    Resource: sdk.MetricResource{
        ServiceName: "http-check",
        ServiceType: "wasm-plugin",
    },
    IngestIdentity: sdk.MetricIngestIdentity{
        Source:       "plugin-metrics",
        ProducerID:   "http-check",
        ProducerKind: "wasm-plugin",
    },
    Metrics: []sdk.Metric{{
        Name:       "http.response_time_ms",
        MetricType: "plugin",
        Kind:       sdk.MetricKindGauge,
        Unit:       "ms",
        Points: []sdk.MetricPoint{{
            Value:              12.5,
            RawValue:           "12.5",
            RawValueType:       sdk.MetricValueTypeDouble,
            ObservedAtUnixNano: uint64(time.Now().UTC().UnixNano()),
        }},
    }},
})

err := sdk.EmitTelemetry(sdk.TelemetryBatch{
    Source:  sdk.TelemetrySource{SourceType: "http-check", SourceInstance: "default"},
    Records: []sdk.TelemetryRecord{record},
})
if err != nil {
    return nil, err
}
```

`NewServiceRadarMetricTelemetryRecordFromBatch` serializes
`serviceradar.metric.v1.MetricBatch` with a dependency-free protobuf encoder so
TinyGo plugins do not need the full Go protobuf runtime. If you already have
encoded protobuf bytes from another generator, use
`NewServiceRadarMetricTelemetryRecord`.

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

### Plugin manifest

`PluginManifest` builds the `plugin.yaml` that ServiceRadar core accepts when a
package is uploaded, and `Validate` mirrors core's own rules so a bad manifest
fails at build time instead of at upload time:

```go
schema := sdk.NewSignalSchemaContribution(
    "com.carverauto.security.scan_activity",
    "1.0.0",
    sdk.SignalSchemaSignalTypeEvent,
    sdk.SignalSchemaPayloadKindOCSFEvent,
).WithOCSF("1.9.0-dev", 6007, 600701)

manifest := sdk.PluginManifest{
    ID:            "security-sample",
    Name:          "Security Sample",
    Version:       "1.0.0",
    Entrypoint:    "run_check",
    Runtime:       sdk.RuntimeWASIPreview1,
    Capabilities:  []string{"get_config", "log", "submit_result", "emit_telemetry"},
    Resources:     map[string]any{"requested_memory_mb": 32},
    Outputs:       sdk.OutputsPluginResult,
    SignalSchemas: []sdk.SignalSchemaContribution{schema},
}

payload, err := manifest.Serialize() // validates, then encodes
```

`NewSignalSchemaContribution` derives the conventional bundle paths
(`schemas/<name>.schema.json`, `display/<name>.display.json`) and the display
contract id/version from the schema id; override any field afterwards.

Validation mirrors `ServiceRadar.Plugins.Manifest` in core, which is strict on
purpose:

- `capabilities`, `runtime`, and `outputs` are checked against core's allowlists.
- `signal_type` must be `event` or `log`; `payload_kind` must be `ocsf_event` or
  `otel_log`.
- Schema and display-contract versions must be semver.
- Bundle paths must be relative `.json` files and may not traverse directories.

The signal schema field set is **closed**. Core rejects any key it does not
recognize with `signal_schemas[i].<key> is not allowed`, so adding a field here
without a matching change in core produces manifests that fail on upload.

### Advisory feed producers
Plugins that produce vulnerability intelligence should emit normalized advisory
batches through the standard plugin result payload. Provider-specific download,
schema validation, pointer JSON handling, archive extraction, and feed-specific
normalization stay inside the plugin. ServiceRadar core consumes only the
generic `serviceradar.advisory_feed.contract.v1` contract.

For large feed snapshots, stage the raw or normalized artifact through the
gateway-mediated artifact API before submitting the advisory batch:

```go
stream, err := sdk.OpenArtifactStream(sdk.ArtifactOpenRequest{
    ObjectKey:   "vulnerability-feeds/example/sha256.json",
    ContentType: "application/json",
})
if err != nil {
    return nil, err
}
if _, err := stream.Write(feedJSON); err != nil {
    _ = stream.Abort()
    return nil, err
}
artifact, err := stream.Commit(sdk.ArtifactCommitRequest{SHA256: feedSHA256})
if err != nil {
    return nil, err
}

batch := sdk.NewAdvisoryFeedBatch(
    "com.example.feed",
    sdk.NewAdvisorySource("example", "normalized"),
    sdk.NewAdvisorySnapshot(artifact.ObjectKey, artifact.SHA256),
).WithAdvisory(
    sdk.NewAdvisoryRecord("CVE-2026-1").
        WithCVE("CVE-2026-1").
        WithSeverity("high").
        WithCoordinate(
            sdk.NewPURLCoordinate("pkg:deb/debian/openssl@3.0.13?arch=amd64").
                WithVersionRange(map[string]any{"fixed_version": "3.0.14"}),
        ),
)

return sdk.Ok("submitted advisory feed").WithAdvisoryFeed(batch), nil
```

Plugins need the `advisory-feed:v1` capability to submit advisory batches and
`artifact-staging:v1` when using the artifact stream helpers. These APIs are
host and agent-gateway mediated; plugins never receive direct object-store
credentials.

Scheduled feed downloads are declared in the plugin package manifest with
`producer_schedules`. The platform persists the declaration, renders operator
settings, and dispatches runs through `plugin.run_action`; the plugin keeps all
provider-specific fetch, checksum, archive, and normalization logic.

```go
schedule := sdk.NewProducerScheduleContract(
    "daily_advisory_refresh",
    "Refresh advisory feed",
    "advisory.refresh",
).
    WithCadence(86_400, 3_600, 2_592_000).
    WithJitterSeconds(120).
    WithCredentialRequirements(map[string]any{"refs": []string{"feed_api_token"}}).
    WithPayloadTemplate(map[string]any{"feed_key": "primary"})
```

Plugins that declare schedules should include `producer-schedule:v1` in their
manifest capabilities. The scheduled invocation payload uses
`serviceradar.producer_schedule_run.v1`.

### Run overrides and action-emitted events
Plugin runs are stateless, so an action that needs to leave state for later
scheduled runs (an injected demo fault, a maintenance window) returns a run
override in its result. The action descriptor must declare a maximum duration;
ServiceRadar clamps every override to it and ignores overrides from actions
without one.

```go
descriptor := sdk.NewActionDescriptor("inject_jam", "Inject conveyor jam", scope).
    WithMaxOverrideDuration(900)

result := sdk.NewActionResult(sdk.ActionStatusSucceeded).
    SetRunOverride("fault-jam-7", "conveyor_jam", "conveyor-7", 10*time.Minute,
        map[string]any{"severity": "critical"}).
    EndRunOverride("fault-saturation-2")

// Opening event of the fault, from the action entrypoint.
err := sdk.EmitOCSFEvent(sdk.NewOCSFEventLogActivity("conveyor jam", sdk.SeverityCritical))
```

Scheduled runs read the overrides the host delivers:

```go
overrides, err := sdk.RunOverrides()
for _, o := range overrides {
    switch {
    case o.Expired:
        // Delivered once more after expiry; emit the resolving event.
    case o.ActiveAt(time.Now()):
        // Apply o.Kind / o.Target / o.Params to this run.
    }
}
```

An expired override keeps being delivered until a run that received it submits
a result. `EmitOCSFEvent` needs the `emit_telemetry` capability and works from
both scheduled runs and action entrypoints.

### Context-aware I/O
Context variants exist for host I/O to match Go expectations:
- HTTP: `HTTP.DoContext`, `HTTP.GetContext`, `HTTP.PostContext`
- TCP: `TCPDialContext`, `(*TCPConn).ReadContext`, `(*TCPConn).WriteContext`
- UDP: `UDPSendToContext`
- WebSocket: `WebSocketDialContext`, `(*WebSocketConn).SendContext`, `(*WebSocketConn).RecvContext`

These currently check `ctx.Err()` before the host call (TinyGo/Wasm is synchronous), but give you a stable API if cancellation support is added later.

### HTTP response headers
The default response mode, `sdk.ResponseModeStatusBody`, returns only the
status and body. Set `sdk.ResponseModeEnvelope` to also receive the response
headers; the host joins repeated values with commas.

```go
resp, err := sdk.HTTP.DoContext(ctx, sdk.HTTPRequest{
    URL:          "https://api.example.com/v1/devices",
    ResponseMode: sdk.ResponseModeEnvelope,
})
if err != nil {
    return nil, err
}
if resp.Status == http.StatusTooManyRequests {
    if wait, ok := resp.RetryAfter(); ok { // delta-seconds or HTTP-date
        return sdk.Warning(fmt.Sprintf("rate limited, retry in %s", wait)), nil
    }
}
contentType := resp.Header("content-type") // case-insensitive
```

### Unary gRPC
`sdk.GRPC.Unary` makes one unary call through the host's `grpc_unary` import.
The message is the serialized request protobuf, so the SDK carries no protobuf
runtime; encode and decode with whatever generator suits TinyGo.

```go
resp, err := sdk.GRPC.Unary(ctx, sdk.GRPCRequest{
    TargetHost: "device.example.com",
    TargetPort: 9200,
    Method:     "/example.v1.Device/Handle",
    Metadata:   map[string]string{"x-request-id": "req-0001"},
    Message:    requestBytes,
    TimeoutMS:  5000,
    Transport:  sdk.GRPCTransportTLS, // default; sdk.GRPCTransportH2C for cleartext
})
var status *sdk.GRPCStatusError
if errors.As(err, &status) {
    // Completed with a non-OK status; resp still carries headers and trailers.
    return sdk.Critical(fmt.Sprintf("rpc failed: %s", status.Code)), nil
}
if err != nil {
    return nil, err // host policy error (sdk.HostError)
}
```

The manifest must declare the `grpc_request` capability
(`sdk.CapabilityGRPCRequest`). The host checks the destination against
`allowed_domains`, `allowed_networks` and `allowed_ports` before dialing,
allows `h2c` only inside `allowed_networks`, caps responses at 4 MiB, and
rejects reserved and `grpc-*` metadata keys. Dial failures arrive as status
`UNAVAILABLE`.

### Credential broker grants
Target contexts carry credential broker grants; the host injects the
credential into matching outbound requests, so the plugin never reads the
secret. The `sdk.CredentialInject*` constants name the accepted inject types.
`OAuth2ClientCredentialsInject` builds an `oauth2_client_credentials` spec with
the exact keys the host reads:

```go
inject, err := sdk.NewOAuth2ClientCredentialsInject("auth.example.com", 443, "/oauth2/token").
    WithScope("devices.read").
    Inject()
// {"type":"oauth2_client_credentials","token_method":"POST","token_host":"auth.example.com",
//  "token_port":"443","token_path":"/oauth2/token","field_client_id":"client_id",
//  "field_client_secret":"client_secret","fixed_grant_type":"client_credentials",
//  "fixed_scope":"devices.read"}
```

`field_<credential field>` maps a stored credential field to a token form
field; `fixed_<form field>` sends a literal value. See
`fixtures/credential_grant_oauth2_client_credentials.json` for a full grant.

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

Pass `GRPCHandler` to serve `grpc_unary` calls the same way; without one,
gRPC calls fail with host error `-4`.

The local host emulates `oauth2_client_credentials` grants itself. Pass the
grants and the credentials, and a request inside a grant's allow scope reaches
your HTTP handler with `Authorization: Bearer <sdk.LocalOAuth2BearerToken(grant)>`,
a synthetic token derived from the grant identity only. A covered request the
grant cannot authorize (missing credential field, outside the inject target,
insecure TLS without opt-in) is denied with `-2`, as on the agent:

```dotenv
SERVICERADAR_CREDENTIAL_CLIENT_ID=local-client
SERVICERADAR_CREDENTIAL_CLIENT_SECRET=local-client-secret
```

```go
capture, err := sdk.RunLocalHost(sdk.LocalHostOptions{
    ConfigJSON:       runtimeConfig,
    HTTPHandler:      newLocalBroker(),
    CredentialGrants: target.CredentialGrants(),
    Credentials:      inputs.Credentials(),
}, runPlugin)
```

The HTTP callback is the trusted local host adapter. It should enforce the same
exact endpoint grant, credential injection, redirects, TLS, and response bounds
as production. Do not add credentials to plugin config, action input, logs, or
results. A successful local run does not bypass production package admission.

Artifact staging is off by default in a local run, so `OpenArtifactStream`
fails the same way it does on an agent without a gateway uploader. Set
`ArtifactDir` to exercise it: each committed artifact is written to
`<ArtifactDir>/<object key>` with owner-only permissions, after the same object
key, SHA-256 and size checks the agent applies, and is listed in
`capture.Artifacts`. Aborted and uncommitted streams leave nothing behind. The
commit response returns the object key unchanged; the agent gateway scopes it
under the agent and assignment instead. Staged artifacts can hold whatever the
plugin retrieved, device configs included, so point `ArtifactDir` outside any
repository.

```go
capture, err := sdk.RunLocalHost(sdk.LocalHostOptions{
    ConfigJSON:  runtimeConfig,
    HTTPHandler: newLocalBroker(credentials),
    ArtifactDir: os.Getenv("SERVICERADAR_LOCAL_ARTIFACT_DIR"),
}, runPlugin)
for _, artifact := range capture.Artifacts {
    fmt.Println(artifact.ObjectKey, artifact.Path, artifact.SHA256)
}
```

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
- `grpc_unary`
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
