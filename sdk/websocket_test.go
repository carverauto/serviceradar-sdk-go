package sdk

import (
	"strings"
	"testing"
)

func TestEncodeWebSocketConnectPayloadURLOnly(t *testing.T) {
	payload, err := encodeWebSocketConnectPayload("ws://camera.local/ws", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(payload) != "ws://camera.local/ws" {
		t.Fatalf("unexpected payload: %s", string(payload))
	}
}

func TestEncodeWebSocketConnectPayloadWithHeaders(t *testing.T) {
	payload, err := encodeWebSocketConnectPayload("wss://camera.local/ws", map[string]string{"Authorization": "Basic abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body := string(payload)
	if body == "wss://camera.local/ws" {
		t.Fatalf("expected JSON payload when headers are present")
	}
	if !strings.Contains(body, "\"url\":\"wss://camera.local/ws\"") {
		t.Fatalf("expected URL field in payload: %s", body)
	}
	if !strings.Contains(body, "\"Authorization\":\"Basic abc\"") {
		t.Fatalf("expected Authorization header in payload: %s", body)
	}
}
