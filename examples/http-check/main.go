package main

import (
	"fmt"
	"github.com/carverauto/serviceradar-sdk-go/sdk"
)

type Config struct {
	URL    string  `json:"url"`
	WarnMS float64 `json:"warn_ms"`
	CritMS float64 `json:"crit_ms"`
}

//export run_check
func run_check() {
	sdk.Execute(func() sdk.Result {
		cfg := Config{URL: "https://example.com/health"}
		_ = sdk.GetConfig(&cfg)

		resp, err := sdk.HTTP.Get(cfg.URL)
		if err != nil {
			res := sdk.Critical("HTTP request failed")
			res.EmitEvent(sdk.SeverityCritical, "HTTP request failed", "http_request_failed")
			res.RequestImmediateAlert("http_request_failed")
			return res
		}

		latencyMS := float64(resp.Duration.Milliseconds())
		res := sdk.NewResult()
		res.SetSummary(fmt.Sprintf("http %d in %.0fms", resp.Status, latencyMS))
		res.ApplyThresholds(latencyMS, floatPtr(cfg.WarnMS), floatPtr(cfg.CritMS))
		res.AddMetric("latency_ms", latencyMS, "ms", &sdk.Thresholds{
			Warn: floatPtr(cfg.WarnMS),
			Crit: floatPtr(cfg.CritMS),
		})
		res.AddStatCard("Latency", fmt.Sprintf("%.0fms", latencyMS), toneForStatus(res.Status))
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

func toneForStatus(status sdk.Status) string {
	switch status {
	case sdk.StatusCritical:
		return "critical"
	case sdk.StatusWarning:
		return "warning"
	default:
		return "success"
	}
}
