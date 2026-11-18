//go:build tinygo

package main

import (
	"github.com/carverauto/serviceradar-sdk-go/v2/sdk"
)

type Config struct {
	LoginURL string  `json:"login_url"`
	APIURL   string  `json:"api_url"`
	WarnMS   float64 `json:"warn_ms"`
	CritMS   float64 `json:"crit_ms"`
}

//export run_check
func run_check() {
	_ = sdk.Execute(func() (*sdk.Result, error) {
		cfg := Config{
			LoginURL: "https://sso.example.com/session",
			APIURL:   "https://app.example.com/app/me",
			WarnMS:   1500,
			CritMS:   5000,
		}
		_ = sdk.LoadConfig(&cfg)

		journey := sdk.NewJourney("http-synthetic-journey")
		journey.Step("login", "POST", cfg.LoginURL).
			WithFollowRedirects(false).
			ExpectStatus(302).
			ExtractCookie("sess", "session_tok").
			ExtractJSONField("/csrf", "csrf_tok").
			Step("api-me", "GET", cfg.APIURL).
			WithHeader("X-CSRF", "{{csrf_tok}}").
			ExpectStatus(200).
			ExpectJSONField("/user/role", "admin").
			ExpectMaxDuration(cfg.WarnMS, cfg.CritMS)

		return journey.Run()
	})
}

func main() {}
