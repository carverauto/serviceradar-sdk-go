//go:build tinygo

package main

import (
	"fmt"
	"time"

	"git.carverauto.dev/carverauto/serviceradar-sdk-go/sdk"
)

type Config struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	TimeoutMS int    `json:"timeout_ms"`
	Payload   string `json:"payload"`
}

//export run_check
func run_check() {
	_ = sdk.Execute(func() (*sdk.Result, error) {
		cfg := Config{Host: "example.com", Port: 53, TimeoutMS: 1000, Payload: "ping"}
		_ = sdk.LoadConfig(&cfg)

		timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
		payload := []byte(cfg.Payload)

		if len(payload) == 0 {
			payload = []byte("ping")
		}

		if err := sdk.UDPSendTo(cfg.Host, uint16(cfg.Port), payload, timeout); err != nil {
			res := sdk.Warning("udp send failed")
			res.EmitEvent(sdk.SeverityWarning, "udp send failed", "udp_send_failed")

			return res, nil
		}

		res := sdk.Ok(fmt.Sprintf("udp ok (%d bytes)", len(payload)))
		res.AddMetric("bytes_sent", float64(len(payload)), "bytes", nil)

		return res, nil
	})
}

func main() {}
