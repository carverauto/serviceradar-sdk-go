//go:build tinygo

package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/carverauto/serviceradar-sdk-go/sdk"
)

type Config struct {
	URL    string  `json:"url"`
	WarnMS float64 `json:"warn_ms"`
	CritMS float64 `json:"crit_ms"`
}

type httpBody struct {
	Status string `json:"status"`
}

//export run_check
func run_check() {
	_ = sdk.Execute(func() (*sdk.Result, error) {
		cfg := Config{URL: "https://example.com/health"}
		_ = sdk.GetConfig(&cfg)

		resp, err := sdk.HTTP.Get(cfg.URL)
		if err != nil {
			res := sdk.Critical("http request failed")
			res.EmitEvent(sdk.SeverityCritical, "http request failed", "http_request_failed")
			res.RequestImmediateAlert("http_request_failed")

			return res, nil
		}

		latencyMS := float64(resp.Duration.Milliseconds())

		res := sdk.NewResult()
		res.SetSummary(fmt.Sprintf("http %d in %.0fms", resp.Status, latencyMS))
		res.ApplyThresholds(latencyMS, floatPtr(cfg.WarnMS), floatPtr(cfg.CritMS))
		res.AddMetric("latency_ms", latencyMS, "ms", &sdk.Thresholds{
			Warn: floatPtr(cfg.WarnMS),
			Crit: floatPtr(cfg.CritMS),
		})

		// Widgets
		res.AddStatCard("Latency", fmt.Sprintf("%.0fms", latencyMS), toneForStatus(res.Status))

		table := map[string]string{
			"Status": fmt.Sprintf("%d", resp.Status),
			"URL":    cfg.URL,
		}

		res.AddTable(table, "full")

		res.AddSparkline("Latency (ms)", sparklineSeries(latencyMS), toneForStatus(res.Status))

		markdown := fmt.Sprintf("**Health Check**\n\n- URL: `%s`\n- Status: `%d`\n- Latency: `%.0fms`",
			cfg.URL, resp.Status, latencyMS)
		res.AddMarkdown(markdown)

		// Optional: parse body status if JSON is returned.
		if len(resp.Body) > 0 && strings.Contains(strings.ToLower(resp.Headers["content-type"]), "application/json") {
			var body httpBody

			if err := json.Unmarshal(resp.Body, &body); err == nil && body.Status != "" {
				res.AddLabel("body_status", body.Status)
			}
		}

		return res, nil
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

func sparklineSeries(latency float64) []float64 {
	seed := latency

	if seed <= 0 {
		seed = 10
	}

	series := make([]float64, 8)

	for i := range series {
		series[i] = seed + float64((i-3))*2
	}

	return series
}
