package sdk

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func fixtureOAuth2Grant(t *testing.T) CredentialBrokerGrant {
	t.Helper()
	raw, _ := readJSONFixture(t, "credential_grant_oauth2_client_credentials.json")
	var grant CredentialBrokerGrant
	if err := json.Unmarshal(raw, &grant); err != nil {
		t.Fatalf("decode grant fixture: %v", err)
	}
	return grant
}

func TestOAuth2ClientCredentialsInjectMatchesSharedFixture(t *testing.T) {
	grant := fixtureOAuth2Grant(t)

	built, err := NewOAuth2ClientCredentialsInject("auth.example.com", 443, "/oauth2/token").
		WithScope("devices.read").
		Inject()
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	if !reflect.DeepEqual(built, grant.Inject) {
		t.Fatalf("built inject %#v\ndoes not match fixture %#v", built, grant.Inject)
	}
	if grant.InjectType() != CredentialInjectOAuth2ClientCredentials {
		t.Fatalf("InjectType = %q", grant.InjectType())
	}
	if grant.Allow == nil || !reflect.DeepEqual(grant.Allow.Hosts, []string{"api.example.com"}) ||
		!reflect.DeepEqual(grant.Allow.Ports, []int{443}) {
		t.Fatalf("allow = %#v", grant.Allow)
	}
	if err := validateOAuth2ClientCredentialsSpec(grant.InjectSpec()); err != nil {
		t.Fatalf("fixture inject spec is invalid: %v", err)
	}
}

func TestOAuth2ClientCredentialsInjectOptionalKeys(t *testing.T) {
	spec, err := NewOAuth2ClientCredentialsInject("auth.example.com", 8443, "/token").
		WithField("app_id", "client_id").
		WithRequestTarget("get", "api.example.com", "/v1/devices").
		Spec()
	if err == nil {
		t.Fatalf("expected duplicate client_id mapping to fail, got %#v", spec)
	}

	spec, err = NewOAuth2ClientCredentialsInject("auth.example.com", 8443, "/token").
		WithRequestTarget("get", "api.example.com", "/v1/devices").
		Spec()
	if err != nil {
		t.Fatalf("Spec: %v", err)
	}
	for key, want := range map[string]string{
		"token_port": "8443",
		"method":     "GET",
		"host":       "api.example.com",
		"path":       "/v1/devices",
	} {
		if spec[key] != want {
			t.Fatalf("spec[%s] = %q, want %q", key, spec[key], want)
		}
	}
	if _, ok := spec[CredentialInjectKeyAllowInsecureTLS]; ok {
		t.Fatal("allow_insecure_tls set without opt-in")
	}
}

func TestOAuth2ClientCredentialsInjectValidation(t *testing.T) {
	base := NewOAuth2ClientCredentialsInject("auth.example.com", 443, "/oauth2/token")
	cases := map[string]OAuth2ClientCredentialsInject{
		"method":        func() OAuth2ClientCredentialsInject { s := base; s.TokenMethod = "GET"; return s }(),
		"host url":      func() OAuth2ClientCredentialsInject { s := base; s.TokenHost = "https://auth.example.com"; return s }(),
		"port":          func() OAuth2ClientCredentialsInject { s := base; s.TokenPort = 0; return s }(),
		"relative path": func() OAuth2ClientCredentialsInject { s := base; s.TokenPath = "oauth2/token"; return s }(),
		"path query":    func() OAuth2ClientCredentialsInject { s := base; s.TokenPath = "/token?x=1"; return s }(),
		"grant type":    base.WithFixed("grant_type", "password"),
		"field key":     base.WithField("Client Secret", "client_secret_2"),
		"no secret": func() OAuth2ClientCredentialsInject {
			s := base
			s.Fields = map[string]string{"client_id": "client_id"}
			return s
		}(),
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if err := spec.Validate(); !errors.Is(err, errInvalidOAuth2ClientCredentialsInject) {
				t.Fatalf("Validate() = %v, want invalid spec", err)
			}
		})
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("default spec is invalid: %v", err)
	}
	if len(base.Fixed) != 1 {
		t.Fatalf("WithFixed mutated the receiver: %#v", base.Fixed)
	}
}

func TestCredentialBrokerGrantInjectSpecStringifies(t *testing.T) {
	grant := CredentialBrokerGrant{Inject: map[string]any{"type": " Bearer_Token ", "token_port": float64(443), "empty": nil}}
	if grant.InjectType() != CredentialInjectBearerToken {
		t.Fatalf("InjectType = %q", grant.InjectType())
	}
	spec := grant.InjectSpec()
	if spec["token_port"] != "443" {
		t.Fatalf("token_port = %q", spec["token_port"])
	}
	if _, ok := spec["empty"]; ok {
		t.Fatal("nil inject value was kept")
	}
}
