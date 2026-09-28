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

func TestHTTPResponseHeaderIsCaseInsensitive(t *testing.T) {
	resp := &HTTPResponse{Headers: map[string]string{"Content-Type": "application/json", "X-Rate-Limit": "10"}}
	if got := resp.Header("content-type"); got != "application/json" {
		t.Fatalf("Header(content-type) = %q", got)
	}
	if got := resp.Header("X-RATE-LIMIT"); got != "10" {
		t.Fatalf("Header(X-RATE-LIMIT) = %q", got)
	}
	if got := resp.Header("missing"); got != "" {
		t.Fatalf("Header(missing) = %q", got)
	}
	var nilResp *HTTPResponse
	if got := nilResp.Header("content-type"); got != "" {
		t.Fatalf("nil response Header = %q", got)
	}
}

func TestHTTPResponseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := map[string]struct {
		value  string
		want   time.Duration
		wantOK bool
	}{
		"absent":        {"", 0, false},
		"delta seconds": {"120", 2 * time.Minute, true},
		"zero":          {"0", 0, true},
		"http date":     {"Fri, 02 Jan 2026 03:05:35 GMT", 90 * time.Second, true},
		"past date":     {"Thu, 01 Jan 2026 00:00:00 GMT", 0, true},
		"negative":      {"-5", 0, false},
		"fractional":    {"1.5", 0, false},
		"malformed":     {"soon", 0, false},
		"overflow":      {"99999999999999999999", 0, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := &HTTPResponse{Headers: map[string]string{}}
			if tc.value != "" {
				resp.Headers["retry-after"] = tc.value
			}
			got, ok := resp.retryAfterAt(now)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("retryAfterAt = (%s, %v), want (%s, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
