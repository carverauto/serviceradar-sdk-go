package sdk

import (
	"testing"
	"time"
)

func TestNewCameraHTTPClient(t *testing.T) {
	t.Parallel()

	client, err := NewCameraHTTPClient(CameraPluginConfig{
		Host:     " camera.local ",
		Scheme:   "HTTPS",
		Timeout:  "15s",
		Username: "root",
		Password: "secret",
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("expected client, got error %v", err)
	}

	if client.BaseURL != "https://camera.local" {
		t.Fatalf("unexpected base URL: %q", client.BaseURL)
	}
	if client.Timeout != 15*time.Second {
		t.Fatalf("unexpected timeout: %s", client.Timeout)
	}
	if client.AuthHeader != "Basic cm9vdDpzZWNyZXQ=" {
		t.Fatalf("unexpected auth header: %q", client.AuthHeader)
	}
	if client.InsecureSkipVerify {
		t.Fatalf("expected insecure TLS to default false")
	}
}

func TestNewCameraHTTPClientPropagatesInsecureSkipVerify(t *testing.T) {
	t.Parallel()

	client, err := NewCameraHTTPClient(CameraPluginConfig{
		Host:               "camera.local",
		Scheme:             "https",
		InsecureSkipVerify: true,
	}, 3*time.Second)
	if err != nil {
		t.Fatalf("expected client, got error %v", err)
	}
	if !client.InsecureSkipVerify {
		t.Fatalf("expected insecure TLS flag to propagate")
	}
}

func TestCameraHTTPClientURL(t *testing.T) {
	t.Parallel()

	client := &CameraHTTPClient{BaseURL: "https://camera.local"}
	if got := client.URL("/axis-cgi/basicdeviceinfo.cgi"); got != "https://camera.local/axis-cgi/basicdeviceinfo.cgi" {
		t.Fatalf("unexpected camera URL: %q", got)
	}
}
