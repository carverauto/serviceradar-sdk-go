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

func TestMarshalHTTPRequestPayload(t *testing.T) {
	payload := httpRequestPayload{
		Method:             "POST",
		URL:                "https://example.invalid/path?q=1",
		Headers:            map[string]string{"X-Test": "value"},
		Body:               "line\nbody",
		ResponseMode:       "status_body",
		TimeoutMS:          1200,
		InsecureSkipVerify: true,
	}

	got := string(marshalHTTPRequestPayload(payload))
	for _, want := range []string{
		`"method":"POST"`,
		`"url":"https://example.invalid/path?q=1"`,
		`"X-Test":"value"`,
		`"body":"line\nbody"`,
		`"response_mode":"status_body"`,
		`"timeout_ms":1200`,
		`"insecure_skip_verify":true`,
	} {
		if !contains(got, want) {
			t.Fatalf("payload %s missing %s", got, want)
		}
	}
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
