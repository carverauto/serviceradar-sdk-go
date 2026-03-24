package sdk

import (
	"testing"
	"time"
)

func TestDefaultCameraPluginConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultCameraPluginConfig()
	if cfg.Scheme != "http" || !cfg.DiscoverStreams || cfg.CollectEvents || cfg.EventSources != "events" || cfg.Timeout != "10s" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestCameraPluginConfigNormalizedScheme(t *testing.T) {
	t.Parallel()

	cfg := CameraPluginConfig{Scheme: " HTTPS "}
	got, err := cfg.NormalizedScheme()
	if err != nil {
		t.Fatalf("expected normalized scheme, got error %v", err)
	}
	if got != "https" {
		t.Fatalf("expected https, got %q", got)
	}
}

func TestCameraPluginConfigParsedTimeout(t *testing.T) {
	t.Parallel()

	cfg := CameraPluginConfig{Timeout: "15s"}
	if got := cfg.ParsedTimeout(3 * time.Second); got != 15*time.Second {
		t.Fatalf("expected 15s timeout, got %s", got)
	}
}

func TestCameraPluginConfigBasicAuthHeader(t *testing.T) {
	t.Parallel()

	cfg := CameraPluginConfig{Username: "root", Password: "secret"}
	got := cfg.BasicAuthHeader()
	want := "Basic cm9vdDpzZWNyZXQ="
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
