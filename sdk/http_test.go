package sdk

import (
	"bytes"
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
	payload := []byte(`{"status":202,"headers":{"content-type":"application/json","Set-Cookie":"a=1; Expires=Wed, 21 Oct 2026 07:28:00 GMT,b=2"},"set_cookies":["a=1; Expires=Wed, 21 Oct 2026 07:28:00 GMT","b=2"],"body_base64":"` +
		base64.StdEncoding.EncodeToString([]byte(`{"queued":true}`)) +
		`","body_encoding":"base64","trace":{"dns_duration_ns":100,"connect_duration_ns":200,"tls_handshake_duration_ns":300,"ttfb_duration_ns":400,"transfer_duration_ns":500,"total_duration_ns":1500,"connection_reused":true,"redirect_count":1,"final_url":"https://example.invalid/final","protocol":"HTTP/2.0","remote_addr":"192.0.2.1:443","hops":[{"url":"https://example.invalid/start","status":302,"ttfb_duration_ns":200},{"url":"https://example.invalid/final","status":202,"ttfb_duration_ns":400}]},"tls":{"version":"TLS1.3","cipher_suite":"TLS_AES_128_GCM_SHA256","alpn":"h2","leaf_subject_cn":"example.invalid","leaf_issuer_cn":"CA","leaf_not_after_unix":1790553600,"leaf_days_remaining":0}}`)

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
	if len(resp.SetCookies) != 2 || resp.SetCookies[0] != "a=1; Expires=Wed, 21 Oct 2026 07:28:00 GMT" {
		t.Fatalf("SetCookies = %#v", resp.SetCookies)
	}
	if vals := resp.HeaderValues("set-cookie"); len(vals) != 2 {
		t.Fatalf("HeaderValues(set-cookie) = %#v", vals)
	}
	if vals := resp.HeaderValues("Content-Type"); len(vals) != 1 || vals[0] != "application/json" {
		t.Fatalf("HeaderValues(Content-Type) = %#v", vals)
	}
	if resp.Trace == nil || !resp.Trace.ConnectionReused || resp.Trace.RedirectCount != 1 || len(resp.Trace.Hops) != 2 {
		t.Fatalf("Trace = %#v", resp.Trace)
	}
	if resp.TLS == nil || resp.TLS.Version != "TLS1.3" || resp.TLS.LeafDaysRemaining == nil || *resp.TLS.LeafDaysRemaining != 0 || resp.TLS.LeafNotAfterUnix == nil || *resp.TLS.LeafNotAfterUnix != 1790553600 {
		t.Fatalf("TLS = %#v", resp.TLS)
	}

	noLeafPayload := []byte(`{"status":200,"body_base64":"","tls":{"version":"TLS1.3","cipher_suite":"TLS_AES_128_GCM_SHA256"}}`)
	noLeafResp, err := decodeEnvelopeHTTPResponse(noLeafPayload, time.Millisecond)
	if err != nil {
		t.Fatalf("decodeEnvelopeHTTPResponse(noLeaf) error: %v", err)
	}
	if noLeafResp.TLS == nil || noLeafResp.TLS.Version != "TLS1.3" || noLeafResp.TLS.LeafNotAfterUnix != nil || noLeafResp.TLS.LeafDaysRemaining != nil {
		t.Fatalf("TLS without leaf cert should have nil expiry fields, got %#v", noLeafResp.TLS)
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
	if contains(got, "follow_redirects") {
		t.Fatalf("nil FollowRedirects must be omitted from JSON: %s", got)
	}

	payload.FollowRedirects = Bool(false)
	gotFalse := string(marshalHTTPRequestPayload(payload))
	if !contains(gotFalse, `"follow_redirects":false`) {
		t.Fatalf("payload %s missing follow_redirects:false", gotFalse)
	}

	payload.FollowRedirects = Bool(true)
	gotTrue := string(marshalHTTPRequestPayload(payload))
	if !contains(gotTrue, `"follow_redirects":true`) {
		t.Fatalf("payload %s missing follow_redirects:true", gotTrue)
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

func TestDecodeStatusBodyHTTPResponseCopiesPayload(t *testing.T) {
	raw := []byte("200\nsecret-first-body")
	resp, ok, err := decodeStatusBodyHTTPResponse(raw, time.Millisecond)
	if err != nil || !ok {
		t.Fatalf("decodeStatusBodyHTTPResponse() = (%v, %v), want ok", ok, err)
	}
	copy(raw, bytes.Repeat([]byte{'X'}, len(raw)))
	if string(resp.Body) != "secret-first-body" {
		t.Fatalf("resp.Body aliased caller buffer: got %q, want %q", string(resp.Body), "secret-first-body")
	}
}

func TestHTTPClientScratchBufferReuseAndClear(t *testing.T) {
	client := &HTTPClient{MaxResponseBytes: 1024}

	buf1 := client.acquireResponseBuffer(1024)
	if len(buf1) != 1024 {
		t.Fatalf("len(buf1) = %d, want 1024", len(buf1))
	}
	copy(buf1, "sensitive-response-payload")
	client.releaseResponseBuffer(buf1, 26)

	buf2 := client.acquireResponseBuffer(1024)
	if &buf1[0] != &buf2[0] {
		t.Fatal("expected acquireResponseBuffer to reuse released scratch buffer")
	}
	for i := 0; i < 26; i++ {
		if buf2[i] != 0 {
			t.Fatalf("released scratch buffer not cleared at index %d: %d", i, buf2[i])
		}
	}
	client.releaseResponseBuffer(buf2, 0)

	bufSmaller := client.acquireResponseBuffer(256)
	if len(bufSmaller) != 256 || &bufSmaller[0] != &buf2[0] {
		t.Fatal("expected smaller acquisition to slice existing scratch buffer")
	}
	client.releaseResponseBuffer(bufSmaller, 0)

	bufLarger := client.acquireResponseBuffer(2048)
	if len(bufLarger) != 2048 || &bufLarger[0] == &bufSmaller[0] {
		t.Fatal("expected larger acquisition to allocate larger buffer")
	}
	client.releaseResponseBuffer(bufLarger, 0)
	if cap(client.scratchBuf) != 2048 {
		t.Fatalf("cap(scratchBuf) = %d, want 2048", cap(client.scratchBuf))
	}
}

func TestDefaultHTTPClientMaxResponseBytesIs16MiB(t *testing.T) {
	const want = 16 * 1024 * 1024
	if MaxHTTPResponseBytes != want {
		t.Fatalf("MaxHTTPResponseBytes = %d, want %d", MaxHTTPResponseBytes, want)
	}
	if HTTP.MaxResponseBytes != want {
		t.Fatalf("HTTP.MaxResponseBytes = %d, want %d", HTTP.MaxResponseBytes, want)
	}
}
