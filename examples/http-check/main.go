//go:build tinygo

package main

import (
	"fmt"

	"git.carverauto.dev/carverauto/serviceradar-sdk-go/sdk"
)

type Config struct {
	URL    string  `json:"url"`
	WarnMS float64 `json:"warn_ms"`
	CritMS float64 `json:"crit_ms"`
}

//export run_check
func run_check() {
	_ = sdk.Execute(func() (*sdk.Result, error) {
		cfg := Config{URL: "https://example.com/health"}
		_ = sdk.LoadConfig(&cfg)

		resp, err := sdk.HTTP.Get(cfg.URL)
		if err != nil {
			res := sdk.Critical("HTTP request failed")
			res.EmitEvent(sdk.SeverityCritical, "HTTP request failed", "http_request_failed")
			res.RequestImmediateAlert("http_request_failed")

			return res, nil
		}

		latencyMS := float64(resp.Duration.Milliseconds())
		thresholds := sdk.Thresholds(cfg.WarnMS, cfg.CritMS)

		res := sdk.NewResult()

		res.SetSummary(fmt.Sprintf("http %d in %.0fms", resp.Status, latencyMS))
		res.ApplyThresholds(latencyMS, thresholds.Warn, thresholds.Crit)
		res.AddMetric("latency_ms", latencyMS, "ms", thresholds)

		res.AddStatCard("Latency", fmt.Sprintf("%.0fms", latencyMS), toneForStatus(res.Status))

		return res, nil
	})
}

func main() {}

func toneForStatus(status sdk.Status) string {
	switch status {
	case sdk.StatusOK:
		return "success"
	case sdk.StatusCritical:
		return "critical"
	case sdk.StatusWarning:
		return "warning"
	case sdk.StatusUnknown:
		return "neutral"
	default:
		return "success"
	}
}
