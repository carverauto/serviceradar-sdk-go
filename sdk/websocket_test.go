package sdk

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestWebSocketConnRequiresHandle(t *testing.T) {
	t.Parallel()

	var conn *WebSocketConn
	if err := conn.Send([]byte("hello"), time.Second); !errors.Is(err, errWebSocketConnNotInitialized) {
		t.Fatalf("expected websocket handle error, got %v", err)
	}

	if _, err := conn.Recv(make([]byte, 16), time.Second); !errors.Is(err, errWebSocketConnNotInitialized) {
		t.Fatalf("expected websocket recv handle error, got %v", err)
	}
}

func TestEncodeWebSocketDialRequestWithoutHeadersUsesRawURL(t *testing.T) {
	t.Parallel()

	data, err := encodeWebSocketDialRequest(WebSocketDialRequest{URL: " wss://protect.local/ws "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := string(data); got != "wss://protect.local/ws" {
		t.Fatalf("unexpected payload %q", got)
	}
}

func TestEncodeWebSocketDialRequestWithHeadersUsesJSONPayload(t *testing.T) {
	t.Parallel()

	data, err := encodeWebSocketDialRequest(WebSocketDialRequest{
		URL: "wss://protect.local/ws",
		Headers: map[string]string{
			"Cookie":    " TOKEN=abc ",
			"X-API-Key": " secret ",
			"":          "ignored",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload WebSocketDialRequest
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("expected JSON payload, got error: %v", err)
	}

	if payload.URL != "wss://protect.local/ws" {
		t.Fatalf("unexpected url %q", payload.URL)
	}
	if payload.Headers["Cookie"] != "TOKEN=abc" {
		t.Fatalf("unexpected cookie header %q", payload.Headers["Cookie"])
	}
	if payload.Headers["X-API-Key"] != "secret" {
		t.Fatalf("unexpected api key header %q", payload.Headers["X-API-Key"])
	}
	if _, ok := payload.Headers[""]; ok {
		t.Fatalf("expected blank header key to be dropped")
	}
}
