//go:build tinygo

package main

import (
	"github.com/carverauto/serviceradar-sdk-go/v2/sdk"
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

		return buildHTTPResult(
			resp.Status,
			float64(resp.Duration.Milliseconds()),
			cfg.WarnMS,
			cfg.CritMS,
		), nil
	})
}

func main() {}
