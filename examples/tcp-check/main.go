//go:build tinygo

package main

import (
	"fmt"
	"time"

	"code.carverauto.dev/carverauto/serviceradar-sdk-go/sdk"
)

type Config struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	TimeoutMS   int    `json:"timeout_ms"`
	Send        string `json:"send"`
	ExpectBytes int    `json:"expect_bytes"`
}

//export run_check
func run_check() {
	_ = sdk.Execute(func() (*sdk.Result, error) {
		cfg := Config{Host: "example.com", Port: 80, TimeoutMS: 2000}
		_ = sdk.LoadConfig(&cfg)

		timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond

		conn, err := sdk.TCPDial(cfg.Host, uint16(cfg.Port), timeout)
		if err != nil {
			res := sdk.Critical("tcp connect failed")
			res.EmitEvent(sdk.SeverityCritical, "tcp connect failed", "tcp_connect_failed")
			res.RequestImmediateAlert("tcp_connect_failed")

			return res, nil
		}

		defer func() {
			_ = conn.Close()
		}()

		if cfg.Send != "" {
			_, err = conn.Write([]byte(cfg.Send), timeout)
			if err != nil {
				res := sdk.Warning("tcp write failed")
				res.EmitEvent(sdk.SeverityWarning, "tcp write failed", "tcp_write_failed")

				return res, nil
			}
		}

		buf := make([]byte, max(1, cfg.ExpectBytes))

		n, err := conn.Read(buf, timeout)
		if err != nil {
			res := sdk.Warning("tcp read failed")
			res.EmitEvent(sdk.SeverityWarning, "tcp read failed", "tcp_read_failed")

			return res, nil
		}

		res := sdk.Ok(fmt.Sprintf("tcp ok (%d bytes)", n))
		res.AddMetric("bytes_read", float64(n), "bytes", nil)

		return res, nil
	})
}

func main() {}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
