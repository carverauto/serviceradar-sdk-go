# serviceradar-sdk-go

ServiceRadar plugin SDK for Go (TinyGo/Wasm).

## Overview
This SDK lets you write ServiceRadar plugin checkers in Go without handling low-level Wasm host calls. It handles:
- Config decoding from the host
- Result builder for `serviceradar.plugin_result.v1`
- Logging bridge
- HTTP/TCP/UDP proxy wrappers
- Event emission + alert promotion hints

## Install

```bash
go get github.com/carverauto/serviceradar-sdk-go
```

## Example

```go
package main

import (
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
    sdk.Execute(func() sdk.Result {
        var cfg Config
        _ = sdk.GetConfig(&cfg)

        resp, err := sdk.HTTP.Get(cfg.URL)
        if err != nil {
            res := sdk.Critical("request failed")
            res.EmitEvent(sdk.SeverityCritical, "request failed", "http_error")
            res.RequestImmediateAlert("http_error")
            return res
        }

        latency := float64(resp.Duration.Milliseconds())
        res := sdk.NewResult()
        res.SetSummary(fmt.Sprintf("http %d in %.0fms", resp.Status, latency))
        res.ApplyThresholds(latency, floatPtr(cfg.WarnMS), floatPtr(cfg.CritMS))
        res.AddMetric("latency_ms", latency, "ms", &sdk.Thresholds{
            Warn: floatPtr(cfg.WarnMS),
            Crit: floatPtr(cfg.CritMS),
        })
        res.AddStatCard("Latency", fmt.Sprintf("%.0fms", latency), "success")
        return res
    })
}

func main() {}

func floatPtr(v float64) *float64 {
    if v <= 0 {
        return nil
    }
    return &v
}
```

## Examples

- `examples/http-check`: HTTP latency check with thresholds and events
- `examples/tcp-check`: TCP connectivity check with optional write/read
- `examples/udp-check`: UDP send check with bytes-sent metric
- `examples/widgets-check`: HTTP check demonstrating stat card, table, sparkline, and markdown widgets

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

The SDK wraps these functions and exports `alloc`/`dealloc` for host memory access.

## Event and Alert Hints

The SDK emits optional fields in the result payload to support event promotion:
- `events`: list of OCSF Event Log Activity objects
- `alert_hint`: boolean flag for immediate promotion
- `condition_id`: string used for de-duplication and auto-clear logic

These fields are ignored safely by older control-plane builds.
