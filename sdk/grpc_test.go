package sdk

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func fixtureRequest() GRPCRequest {
	return GRPCRequest{
		TargetHost: "device.example.com",
		TargetPort: 9200,
		Authority:  "device.example.com:9200",
		Method:     "/example.v1.Device/Handle",
		Metadata: map[string]string{
			"x-request-id": "req-0001",
			"trace-bin":    "AAECAw==",
		},
		Message:          []byte("\n\x06dev-01"),
		TimeoutMS:        5000,
		Transport:        GRPCTransportTLS,
		TLS:              &GRPCTLSConfig{ServerName: "device.example.com"},
		MaxResponseBytes: 65536,
	}
}

func readJSONFixture(t *testing.T, name string) ([]byte, any) {
	t.Helper()
	raw, err := os.ReadFile("../fixtures/" + name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return raw, decoded
}

func TestGRPCRequestMatchesSharedFixture(t *testing.T) {
	_, want := readJSONFixture(t, "grpc_unary_request.json")

	payload, err := newGRPCRequestPayload(fixtureRequest())
	if err != nil {
		t.Fatalf("newGRPCRequestPayload: %v", err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var got any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("encoded request\n%s\ndoes not match fixture %v", encoded, want)
	}
}

func TestGRPCRequestDefaultsToTLSAndOmitsOptionalFields(t *testing.T) {
	payload, err := newGRPCRequestPayload(GRPCRequest{
		TargetHost: "192.0.2.10",
		TargetPort: 9200,
		Method:     "/example.v1.Device/Handle",
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"target_host":"192.0.2.10","target_port":9200,"method":"/example.v1.Device/Handle",` +
		`"message_base64":"","transport":"tls"}`
	if string(encoded) != want {
		t.Fatalf("encoded = %s, want %s", encoded, want)
	}
}

func TestGRPCRequestRejectsInvalidShapes(t *testing.T) {
	base := fixtureRequest()
	cases := map[string]struct {
		mutate func(*GRPCRequest)
		want   error
	}{
		"missing host":   {func(r *GRPCRequest) { r.TargetHost = " " }, errGRPCMissingTarget},
		"zero port":      {func(r *GRPCRequest) { r.TargetPort = 0 }, errGRPCInvalidPort},
		"high port":      {func(r *GRPCRequest) { r.TargetPort = 70000 }, errGRPCInvalidPort},
		"no slash":       {func(r *GRPCRequest) { r.Method = "example.v1.Device/Handle" }, errGRPCInvalidMethod},
		"no method name": {func(r *GRPCRequest) { r.Method = "/example.v1.Device/" }, errGRPCInvalidMethod},
		"extra segment":  {func(r *GRPCRequest) { r.Method = "/a/b/c" }, errGRPCInvalidMethod},
		"transport":      {func(r *GRPCRequest) { r.Transport = "http" }, errGRPCTransport},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := base
			tc.mutate(&req)
			if _, err := newGRPCRequestPayload(req); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDecodeGRPCResponseOKFixture(t *testing.T) {
	raw, _ := readJSONFixture(t, "grpc_unary_response_ok.json")

	resp, err := decodeGRPCResponse(raw, time.Millisecond)
	if err != nil {
		t.Fatalf("decodeGRPCResponse: %v", err)
	}
	if resp.Status != GRPCCodeOK || resp.StatusMessage != "" {
		t.Fatalf("status = %v %q", resp.Status, resp.StatusMessage)
	}
	if string(resp.Message) != "\n\x02ok" {
		t.Fatalf("message = %q", resp.Message)
	}
	if got := resp.Headers["content-type"]; len(got) != 1 || got[0] != "application/grpc" {
		t.Fatalf("headers = %#v", resp.Headers)
	}
	if got := resp.Trailers["x-handled-by"]; len(got) != 1 || got[0] != "handler-1" {
		t.Fatalf("trailers = %#v", resp.Trailers)
	}
	if resp.Duration != time.Millisecond {
		t.Fatalf("duration = %s", resp.Duration)
	}
}

func TestDecodeGRPCResponseErrorFixture(t *testing.T) {
	raw, _ := readJSONFixture(t, "grpc_unary_response_error.json")

	resp, err := decodeGRPCResponse(raw, 0)
	var statusErr *GRPCStatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %v, want *GRPCStatusError", err)
	}
	if statusErr.Code != GRPCCodeNotFound || statusErr.Message != "device dev-01 not found" {
		t.Fatalf("status error = %#v", statusErr)
	}
	if got := statusErr.Trailers["x-error-detail"]; len(got) != 1 || got[0] != "unknown-device" {
		t.Fatalf("trailers = %#v", statusErr.Trailers)
	}
	if statusErr.Error() != "grpc status NOT_FOUND: device dev-01 not found" {
		t.Fatalf("Error() = %q", statusErr.Error())
	}
	if resp == nil || resp.Status != GRPCCodeNotFound || len(resp.Message) != 0 {
		t.Fatalf("response = %#v, want the non-OK response alongside the error", resp)
	}
}

func TestGRPCCodeString(t *testing.T) {
	if GRPCCodeUnavailable.String() != "UNAVAILABLE" || GRPCCodeUnauthenticated.String() != "UNAUTHENTICATED" {
		t.Fatal("unexpected code names")
	}
	if GRPCCode(42).String() != "CODE(42)" {
		t.Fatalf("unknown code = %q", GRPCCode(42).String())
	}
}

func TestGRPCResponseBufferSize(t *testing.T) {
	client := &GRPCClient{MaxResponseBytes: 3000}
	if got := client.responseBufferSize(0); got != 4000+grpcResponseEnvelopeOverhead {
		t.Fatalf("buffer = %d", got)
	}
	if got := client.responseBufferSize(300); got != 400+grpcResponseEnvelopeOverhead {
		t.Fatalf("request-limited buffer = %d", got)
	}
	if got := (&GRPCClient{}).responseBufferSize(0); got < MaxGRPCResponseBytes {
		t.Fatalf("default buffer = %d", got)
	}
}
