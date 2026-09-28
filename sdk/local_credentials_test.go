//go:build !tinygo

package sdk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type localGrantCall struct {
	url     string
	headers map[string]string
}

func runLocalGrantRequests(
	t *testing.T,
	credentials map[string]string,
	grants []CredentialBrokerGrant,
	requests ...HTTPRequest,
) ([]localGrantCall, []error) {
	t.Helper()
	var calls []localGrantCall
	errs := make([]error, len(requests))
	_, err := RunLocalHost(LocalHostOptions{
		ConfigJSON:       []byte(`{}`),
		CredentialGrants: grants,
		Credentials:      credentials,
		HTTPHandler: func(_ context.Context, request HTTPRequest) (*HTTPResponse, error) {
			calls = append(calls, localGrantCall{url: request.URL, headers: request.Headers})
			return &HTTPResponse{Status: 200}, nil
		},
	}, func() error {
		for i, request := range requests {
			_, errs[i] = HTTP.Do(request)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunLocalHost: %v", err)
	}
	return calls, errs
}

func localOAuth2Credentials(t *testing.T) map[string]string {
	t.Helper()
	inputs, err := LoadLocalInputs(LocalInputOptions{
		ConfigJSON: []byte(`{}`),
		Environment: []string{
			"SERVICERADAR_CREDENTIAL_CLIENT_ID=local-client",
			"SERVICERADAR_CREDENTIAL_CLIENT_SECRET=local-client-secret",
		},
	})
	if err != nil {
		t.Fatalf("LoadLocalInputs: %v", err)
	}
	return inputs.Credentials()
}

func TestLocalHostInjectsOAuth2ClientCredentialsBearer(t *testing.T) {
	grant := fixtureOAuth2Grant(t)
	token := LocalOAuth2BearerToken(grant)
	credentials := localOAuth2Credentials(t)

	calls, errs := runLocalGrantRequests(t, credentials, []CredentialBrokerGrant{grant},
		HTTPRequest{
			URL:     "https://api.example.com/v1/devices",
			Headers: map[string]string{"authorization": "Bearer guest-supplied", "Accept": "application/json"},
		},
		HTTPRequest{URL: "https://other.example.com/v1/devices"},
		HTTPRequest{Method: "POST", URL: "https://api.example.com/v1/devices"},
	)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if len(calls) != 3 {
		t.Fatalf("handler calls = %d", len(calls))
	}

	covered := calls[0].headers
	if covered["Authorization"] != "Bearer "+token || covered["authorization"] != "" {
		t.Fatalf("covered request headers = %#v", covered)
	}
	if covered["Accept"] != "application/json" {
		t.Fatalf("plugin headers were dropped: %#v", covered)
	}
	for _, value := range covered {
		if strings.Contains(value, "local-client-secret") {
			t.Fatal("client secret reached the outbound request")
		}
	}
	if strings.Contains(token, "local-client") {
		t.Fatal("synthetic token is derived from credential material")
	}
	for _, call := range calls[1:] {
		if _, ok := call.headers["Authorization"]; ok {
			t.Fatalf("request outside the grant allow scope got a bearer: %s", call.url)
		}
	}
}

func TestLocalHostDeniesCoveredRequestsItCannotAuthorize(t *testing.T) {
	grant := fixtureOAuth2Grant(t)
	targeted := fixtureOAuth2Grant(t)
	targeted.Inject["path"] = "/v1/devices"

	cases := map[string]struct {
		credentials map[string]string
		grant       CredentialBrokerGrant
		request     HTTPRequest
	}{
		"missing secret": {
			credentials: map[string]string{"client_id": "local-client"},
			grant:       grant,
			request:     HTTPRequest{URL: "https://api.example.com/v1/devices"},
		},
		"outside inject target": {
			credentials: localOAuth2Credentials(t),
			grant:       targeted,
			request:     HTTPRequest{URL: "https://api.example.com/v1/sites"},
		},
		"insecure tls": {
			credentials: localOAuth2Credentials(t),
			grant:       grant,
			request:     HTTPRequest{URL: "https://api.example.com/v1/devices", InsecureSkipVerify: true},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			calls, errs := runLocalGrantRequests(t, tc.credentials, []CredentialBrokerGrant{tc.grant}, tc.request)
			var hostErr HostError
			if !errors.As(errs[0], &hostErr) || hostErr.Code != hostErrDenied {
				t.Fatalf("error = %v, want host error -2", errs[0])
			}
			if len(calls) != 0 {
				t.Fatal("denied request reached the handler")
			}
		})
	}
}

func TestLocalHostSkipsExpiredAndOtherGrantTypes(t *testing.T) {
	expired := fixtureOAuth2Grant(t)
	expired.ExpiresAt = "2020-01-01T00:00:00Z"
	header := fixtureOAuth2Grant(t)
	header.Inject = map[string]any{"type": CredentialInjectHTTPHeader, "name": "X-Api-Key"}

	calls, errs := runLocalGrantRequests(t, localOAuth2Credentials(t), []CredentialBrokerGrant{expired, header},
		HTTPRequest{URL: "https://api.example.com/v1/devices"})
	if errs[0] != nil {
		t.Fatalf("request: %v", errs[0])
	}
	if len(calls) != 1 || len(calls[0].headers) != 0 {
		t.Fatalf("unexpected injection: %#v", calls)
	}
}
