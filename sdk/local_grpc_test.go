//go:build !tinygo

package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func fixtureGRPCResponse(t *testing.T, name string) GRPCResponse {
	t.Helper()
	raw, _ := readJSONFixture(t, name)
	var payload grpcResponsePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	message, err := base64.StdEncoding.DecodeString(payload.MessageBase64)
	if err != nil {
		t.Fatal(err)
	}
	return GRPCResponse{
		Status:        GRPCCode(payload.GRPCStatus),
		StatusMessage: payload.GRPCMessage,
		Headers:       payload.Headers,
		Trailers:      payload.Trailers,
		Message:       message,
	}
}

func runLocalGRPC(t *testing.T, handler LocalGRPCHandler, req GRPCRequest) (*GRPCResponse, error) {
	t.Helper()
	var resp *GRPCResponse
	var callErr error
	_, err := RunLocalHost(LocalHostOptions{ConfigJSON: []byte(`{}`), GRPCHandler: handler}, func() error {
		resp, callErr = GRPC.Unary(context.Background(), req)
		return nil
	})
	if err != nil {
		t.Fatalf("RunLocalHost: %v", err)
	}
	return resp, callErr
}

func TestLocalHostGRPCRoundTripsSharedFixtures(t *testing.T) {
	raw, _ := readJSONFixture(t, "grpc_unary_request.json")
	want, err := decodeLocalGRPCRequest(raw)
	if err != nil {
		t.Fatalf("decode request fixture: %v", err)
	}
	okResponse := fixtureGRPCResponse(t, "grpc_unary_response_ok.json")

	calls := 0
	resp, err := runLocalGRPC(t, func(ctx context.Context, got GRPCRequest) (GRPCResponse, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Error("handler context has no deadline")
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("handler request = %#v, want %#v", got, want)
		}
		return okResponse, nil
	}, fixtureRequest())
	if err != nil {
		t.Fatalf("Unary: %v", err)
	}
	if calls != 1 {
		t.Fatalf("handler calls = %d", calls)
	}
	if resp.Status != GRPCCodeOK || string(resp.Message) != "\n\x02ok" ||
		!reflect.DeepEqual(resp.Trailers, okResponse.Trailers) {
		t.Fatalf("response = %#v", resp)
	}
}

func TestLocalHostGRPCNonOKStatus(t *testing.T) {
	errorResponse := fixtureGRPCResponse(t, "grpc_unary_response_error.json")

	resp, err := runLocalGRPC(t, func(context.Context, GRPCRequest) (GRPCResponse, error) {
		return errorResponse, nil
	}, fixtureRequest())
	var statusErr *GRPCStatusError
	if !errors.As(err, &statusErr) || statusErr.Code != GRPCCodeNotFound {
		t.Fatalf("error = %v, want NOT_FOUND status", err)
	}
	if resp == nil || !reflect.DeepEqual(resp.Headers, errorResponse.Headers) {
		t.Fatalf("response = %#v", resp)
	}
}

func TestLocalHostGRPCWithoutHandlerIsUnsupported(t *testing.T) {
	_, err := runLocalGRPC(t, nil, fixtureRequest())
	var hostErr HostError
	if !errors.As(err, &hostErr) || hostErr.Code != hostErrNotFound || hostErr.Op != "grpc_unary" {
		t.Fatalf("error = %v, want host error -4", err)
	}
}

func TestLocalHostGRPCHandlerErrorsBecomeUnavailable(t *testing.T) {
	secret := errors.New("dial local-secret@192.0.2.10 refused")
	_, err := runLocalGRPC(t, func(context.Context, GRPCRequest) (GRPCResponse, error) {
		return GRPCResponse{}, secret
	}, fixtureRequest())
	var statusErr *GRPCStatusError
	if !errors.As(err, &statusErr) || statusErr.Code != GRPCCodeUnavailable {
		t.Fatalf("error = %v, want UNAVAILABLE", err)
	}
	if errors.Is(err, secret) || statusErr.Message == secret.Error() {
		t.Fatal("handler error text leaked to the plugin")
	}
}

func TestLocalHostGRPCTimeout(t *testing.T) {
	req := fixtureRequest()
	req.TimeoutMS = 1
	_, err := runLocalGRPC(t, func(ctx context.Context, _ GRPCRequest) (GRPCResponse, error) {
		<-ctx.Done()
		return GRPCResponse{}, ctx.Err()
	}, req)
	var hostErr HostError
	if !errors.As(err, &hostErr) || hostErr.Code != hostErrTimeout {
		t.Fatalf("error = %v, want host error -6", err)
	}
}

func TestLocalHostGRPCEnforcesResponseCap(t *testing.T) {
	req := fixtureRequest()
	req.MaxResponseBytes = 4
	_, err := runLocalGRPC(t, func(context.Context, GRPCRequest) (GRPCResponse, error) {
		return GRPCResponse{Message: []byte("12345")}, nil
	}, req)
	var hostErr HostError
	if !errors.As(err, &hostErr) || hostErr.Code != hostErrTooLarge {
		t.Fatalf("error = %v, want host error -3", err)
	}
}

func TestLocalHostGRPCMetadataRules(t *testing.T) {
	cases := map[string]struct {
		metadata map[string]string
		wantErr  bool
	}{
		"lowercased":      {map[string]string{"X-Request-ID": "req-0002"}, false},
		"grpc prefix":     {map[string]string{"grpc-timeout": "1S"}, true},
		"pseudo header":   {map[string]string{":authority": "api.example.com"}, true},
		"reserved header": {map[string]string{"Content-Type": "application/grpc"}, true},
		"bad binary":      {map[string]string{"trace-bin": "not base64!"}, true},
		"case duplicate":  {map[string]string{"x-a": "1", "X-A": "2"}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := fixtureRequest()
			req.Metadata = tc.metadata
			var seen map[string]string
			_, err := runLocalGRPC(t, func(_ context.Context, got GRPCRequest) (GRPCResponse, error) {
				seen = got.Metadata
				return GRPCResponse{}, nil
			}, req)
			if tc.wantErr {
				var hostErr HostError
				if !errors.As(err, &hostErr) || hostErr.Code != hostErrInvalid {
					t.Fatalf("error = %v, want host error -1", err)
				}
				return
			}
			if err != nil || seen["x-request-id"] != "req-0002" {
				t.Fatalf("err = %v metadata = %#v", err, seen)
			}
		})
	}
}
