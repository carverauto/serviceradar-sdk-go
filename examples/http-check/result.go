package main

import (
	"fmt"

	"github.com/carverauto/serviceradar-sdk-go/v2/sdk"
)

func buildHTTPResult(statusCode int, latencyMS, warnMS, critMS float64) *sdk.Result {
	thresholds := sdk.Thresholds(warnMS, critMS)
	result := sdk.NewResult()

	result.SetSummary(fmt.Sprintf("http %d in %.0fms", statusCode, latencyMS))
	result.ApplyThresholds(latencyMS, thresholds.Warn, thresholds.Crit)
	result.AddStatCard("Latency", fmt.Sprintf("%.0fms", latencyMS), toneForStatus(result.Status))

	return result
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
