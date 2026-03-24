package sdk

import "testing"

func TestWithURLUserInfo(t *testing.T) {
	t.Parallel()

	got := WithURLUserInfo("wss://camera.local/vapix/ws-data-stream?sources=events", "root", "secret")
	want := "wss://root:secret@camera.local/vapix/ws-data-stream?sources=events"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWithURLUserInfoWithoutCredentials(t *testing.T) {
	t.Parallel()

	rawURL := "wss://camera.local/vapix/ws-data-stream?sources=events"
	if got := WithURLUserInfo(rawURL, "", ""); got != rawURL {
		t.Fatalf("expected unchanged URL %q, got %q", rawURL, got)
	}
}
