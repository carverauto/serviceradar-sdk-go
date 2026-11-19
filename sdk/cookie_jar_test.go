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

func TestCookieJarLargeMaxAgeRetentionAndOverflow(t *testing.T) {
	now := time.Unix(1790510400, 0).UTC()
	jar := NewCookieJar()
	jar.nowFn = func() time.Time { return now }

	jar.StoreResponseCookies("https://overflow.example.com/login", []string{
		"long_lived=still_here; Path=/; Max-Age=10000000000",
		"huge_pos=kept; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=99999999999999999999999999999",
		"huge_plus=kept_plus; Path=/; Max-Age=+99999999999999999999999999999",
		"bad_syntax=not_clamped; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT; Max-Age=99999999999999999999999999999abc",
	})

	want := "long_lived=still_here; huge_pos=kept; huge_plus=kept_plus"
	if got := jar.HeaderValue("https://overflow.example.com/check"); got != want {
		t.Fatalf("HeaderValue = %q, want %q", got, want)
	}

	// Negative overflow deletes existing cookie.
	jar.StoreResponseCookies("https://overflow.example.com/logout", []string{
		"long_lived=gone; Path=/; Max-Age=-99999999999999999999999999999",
	})
	wantAfterNeg := "huge_pos=kept; huge_plus=kept_plus"
	if got := jar.HeaderValue("https://overflow.example.com/check"); got != wantAfterNeg {
		t.Fatalf("after negative overflow HeaderValue = %q, want %q", got, wantAfterNeg)
	}
}

func TestCookieJarDoubleDotAndIPv6Brackets(t *testing.T) {
	now := time.Unix(1790510400, 0).UTC()
	jar := NewCookieJar()
	jar.nowFn = func() time.Time { return now }

	jar.StoreResponseCookies("https://sso.example.com/login", []string{
		"double_dot=evil; Domain=..example.com; Path=/",
		"single_dot=ok; Domain=.example.com; Path=/",
	})
	if got := jar.HeaderValue("https://sso.example.com/check"); got != "single_dot=ok" {
		t.Fatalf("sso HeaderValue = %q, want single_dot=ok", got)
	}
	if got := jar.HeaderValue("https://app.example.com/check"); got != "single_dot=ok" {
		t.Fatalf("app HeaderValue = %q, want single_dot=ok", got)
	}

	jar.StoreResponseCookies("https://[::1]/login", []string{
		"v6_host=v1; Path=/",
		"v6_dom=v2; Domain=::1; Path=/",
		"v6_brack=v3; Domain=[::1]; Path=/",
	})
	if got := jar.HeaderValue("https://[::1]/check"); got != "v6_host=v1; v6_dom=v2" {
		t.Fatalf("ipv6 HeaderValue = %q, want \"v6_host=v1; v6_dom=v2\"", got)
	}
}

func TestCookieJarRFC6265CookieDateAlgorithm(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"Thursday, 01-Jan-69 00:00:00 GMT", time.Date(2069, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"Sunday, 06-Nov-94 08:49:37 GMT", time.Date(1994, 11, 6, 8, 49, 37, 0, time.UTC), true},
		{"thu, 01 jan 1970 00:00:00 gmt", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"01-Jan-1970 00:00:00", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"Sun Nov  6 08:49:37 1994", time.Date(1994, 11, 6, 8, 49, 37, 0, time.UTC), true},
		{"Wed, 21 Oct 2026 07:28:60 GMT", time.Time{}, false},
		{"Wed, 31 Feb 2026 07:28:00 GMT", time.Time{}, false},
		{"Wed, 21 Oct 1600 07:28:00 GMT", time.Time{}, false},
		{"Wed, 21 Oct 2026", time.Time{}, false},
	}
	for _, tc := range cases {
		got, ok := parseCookieExpires(tc.in)
		if ok != tc.ok || !got.Equal(tc.want) {
			t.Errorf("parseCookieExpires(%q) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}

	now := time.Unix(1790510400, 0).UTC()
	jar := NewCookieJar()
	jar.nowFn = func() time.Time { return now }
	jar.StoreResponseCookies("https://app.example.com/", []string{
		"keep=1; Path=/; Expires=Thursday, 01-Jan-69 00:00:00 GMT",
		"gone=1; Path=/",
	})
	jar.StoreResponseCookies("https://app.example.com/", []string{
		"gone=0; Path=/; Expires=thu, 01 jan 1970 00:00:00 gmt",
	})
	if got := jar.HeaderValue("https://app.example.com/"); got != "keep=1" {
		t.Fatalf("HeaderValue = %q, want keep=1", got)
	}
}

func TestCookieJarRejectsPublicSuffixDomain(t *testing.T) {
	jar := NewCookieJar()
	jar.StoreResponseCookies("https://a.co.uk/", []string{
		"ps=bad; Domain=co.uk; Path=/",
		"ok=1; Domain=a.co.uk; Path=/",
	})
	if got := jar.HeaderValue("https://b.co.uk/"); got != "" {
		t.Fatalf("public-suffix cookie leaked to sibling registrable domain: %q", got)
	}
	if got := jar.HeaderValue("https://www.a.co.uk/"); got != "ok=1" {
		t.Fatalf("www.a.co.uk HeaderValue = %q, want ok=1", got)
	}

	jar.StoreResponseCookies("https://co.uk/", []string{"self=1; Domain=co.uk; Path=/"})
	if got := jar.HeaderValue("https://co.uk/"); got != "self=1" {
		t.Fatalf("co.uk HeaderValue = %q, want self=1", got)
	}
	if got := jar.HeaderValue("https://a.co.uk/"); got != "ok=1" {
		t.Fatalf("public-suffix host cookie must stay host-only, got %q", got)
	}
}

func TestCookieJarIgnoresSecureCookieFromInsecureOrigin(t *testing.T) {
	jar := NewCookieJar()
	jar.StoreResponseCookies("http://app.example.com/login", []string{
		"sid=attacker; Path=/; Secure",
		"plain=1; Path=/",
	})
	if got := jar.HeaderValue("https://app.example.com/api"); got != "plain=1" {
		t.Fatalf("HeaderValue = %q, want plain=1", got)
	}
}
