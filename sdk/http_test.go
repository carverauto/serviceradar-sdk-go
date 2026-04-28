package sdk

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestDecodeStatusBodyHTTPResponse(t *testing.T) {
	resp, ok, err := decodeStatusBodyHTTPResponse([]byte("200\n{\"ok\":true}"), time.Millisecond)
	if err != nil {
		t.Fatalf("decodeStatusBodyHTTPResponse returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected status/body response")
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, want 200", resp.Status)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Fatalf("body = %q", string(resp.Body))
	}
	if resp.Duration != time.Millisecond {
		t.Fatalf("duration = %s, want 1ms", resp.Duration)
	}
}

func TestDecodeStatusBodyHTTPResponseIgnoresLegacyEnvelope(t *testing.T) {
	_, ok, err := decodeStatusBodyHTTPResponse([]byte(`{"status":200}`), time.Millisecond)
	if err != nil {
		t.Fatalf("decodeStatusBodyHTTPResponse returned error: %v", err)
	}
	if ok {
		t.Fatalf("legacy JSON envelope should not be treated as status/body response")
	}
}

func TestDecodeEnvelopeHTTPResponse(t *testing.T) {
	payload := []byte(`{"status":202,"headers":{"content-type":"application/json"},"body_base64":"` +
		base64.StdEncoding.EncodeToString([]byte(`{"queued":true}`)) +
		`","body_encoding":"base64"}`)

	resp, err := decodeEnvelopeHTTPResponse(payload, 2*time.Millisecond)
	if err != nil {
		t.Fatalf("decodeEnvelopeHTTPResponse returned error: %v", err)
	}
	if resp.Status != 202 {
		t.Fatalf("status = %d, want 202", resp.Status)
	}
	if string(resp.Body) != `{"queued":true}` {
		t.Fatalf("body = %q", string(resp.Body))
	}
	if resp.Headers["content-type"] != "application/json" {
		t.Fatalf("headers = %#v", resp.Headers)
	}
	if resp.Duration != 2*time.Millisecond {
		t.Fatalf("duration = %s, want 2ms", resp.Duration)
	}
}
