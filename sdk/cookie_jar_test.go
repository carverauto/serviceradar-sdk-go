package sdk

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCookieJarRFC6265ScopingAndRedaction(t *testing.T) {
	now := time.Unix(1790510400, 0).UTC()
	jar := NewCookieJar()
	jar.nowFn = func() time.Time { return now }

	jar.StoreResponseCookies("https://sso.example.com/auth/login", []string{
		"host_sid=secret-host-val; Path=/; HttpOnly; Secure",
		"domain_tok=secret-domain-val; Domain=.example.com; Path=/app; Secure",
		"evil_cross=bad; Domain=other.com; Path=/",
		"evil_tld=bad; Domain=com; Path=/",
	})

	if got := jar.HeaderValue("https://sso.example.com/auth/step2"); got != "host_sid=secret-host-val" {
		t.Fatalf("sso header = %q", got)
	}
	if got := jar.HeaderValue("https://app.example.com/app/home"); got != "domain_tok=secret-domain-val" {
		t.Fatalf("app.example.com header = %q", got)
	}
	if got := jar.HeaderValue("http://app.example.com/app/home"); got != "" {
		t.Fatalf("Secure cookie leaked over http: %q", got)
	}
	if got := jar.HeaderValue("https://evil-example.com/app/home"); got != "" {
		t.Fatalf("cookie leaked across domain suffix boundary: %q", got)
	}
	if got := jar.HeaderValue("https://app.example.com/application"); got != "" {
		t.Fatalf("cookie leaked across path prefix boundary: %q", got)
	}

	// Verify String/GoString redaction.
	formatted := fmt.Sprintf("%s %#v", jar, jar)
	if strings.Contains(formatted, "secret-host-val") || strings.Contains(formatted, "secret-domain-val") {
		t.Fatalf("CookieJar formatted secret value: %s", formatted)
	}
	for _, c := range jar.CookiesForURL("https://sso.example.com/auth/step2") {
		cs := fmt.Sprintf("%s %#v", c, c)
		if strings.Contains(cs, "secret-host-val") {
			t.Fatalf("Cookie formatted secret value: %s", cs)
		}
	}
}

func TestCookieJarMaxAgePrecedenceAndDateFormats(t *testing.T) {
	now := time.Unix(1790510400, 0).UTC()
	jar := NewCookieJar()
	jar.nowFn = func() time.Time { return now }

	jar.StoreResponseCookies("https://app.example.com/login", []string{
		"a=1; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=60",
		"b=2; Path=/; Expires=Sun Nov  6 08:49:37 2033",
		"c=3; Path=/; Expires=Sunday, 06-Nov-33 08:49:37 GMT",
		"d=4; Path=/; Max-Age=-1",
	})

	if got := jar.HeaderValue("https://app.example.com/"); got != "a=1; b=2; c=3" {
		t.Fatalf("HeaderValue = %q", got)
	}

	// Advance 61s -> 'a' expires.
	now = now.Add(61 * time.Second)
	if got := jar.HeaderValue("https://app.example.com/"); got != "b=2; c=3" {
		t.Fatalf("after 61s HeaderValue = %q", got)
	}

	// Max-Age=0 deletes 'b' despite future Expires.
	jar.StoreResponseCookies("https://app.example.com/logout", []string{
		"b=gone; Path=/; Expires=Wed, 21 Oct 2026 07:28:00 GMT; Max-Age=0",
	})
	if got := jar.HeaderValue("https://app.example.com/"); got != "c=3" {
		t.Fatalf("after Max-Age=0 HeaderValue = %q", got)
	}
}
