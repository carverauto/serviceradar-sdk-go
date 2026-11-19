package sdk

import (
	"errors"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/carverauto/serviceradar-sdk-go/v2/sdk/internal/publicsuffix"
)

const maxCookieDurationSecs = math.MaxInt64 / int64(time.Second)

// Cookie represents one RFC 6265 cookie stored in an ephemeral CookieJar.
// Secret values are redacted from String/GoString formatting.
type Cookie struct {
	Name       string
	Value      string
	Domain     string
	HostOnly   bool
	Path       string
	Secure     bool
	HTTPOnly   bool
	HasExpiry  bool
	ExpiresAt  time.Time
	Persistent bool
	createdSeq uint64
}

// String redacts the cookie value so accidental formatting cannot leak secrets.
func (c Cookie) String() string {
	return "Cookie{name=" + c.Name + ",domain=" + c.Domain + ",path=" + c.Path + ",value=[REDACTED]}"
}

// GoString redacts the cookie value for %#v formatting.
func (c Cookie) GoString() string {
	return c.String()
}

// CookieJar is an ephemeral, in-memory RFC 6265 cookie jar scoped to one
// journey execution. It is never serialized or logged.
type CookieJar struct {
	cookies []Cookie
	seq     uint64
	nowFn   func() time.Time
}

// NewCookieJar returns a new empty ephemeral CookieJar.
func NewCookieJar() *CookieJar {
	return &CookieJar{}
}

// String redacts all stored cookies so formatting never leaks session material.
func (j *CookieJar) String() string {
	return "CookieJar{redacted}"
}

// GoString redacts all stored cookies for %#v formatting.
func (j *CookieJar) GoString() string {
	return "CookieJar{redacted}"
}

func (j *CookieJar) now() time.Time {
	if j != nil && j.nowFn != nil {
		return j.nowFn().UTC()
	}
	return time.Now().UTC()
}

// StoreResponseCookies parses and stores Set-Cookie headers returned by rawURL
// according to RFC 6265 domain, path, Secure, Max-Age, and Expires rules.
func (j *CookieJar) StoreResponseCookies(rawURL string, setCookies []string) {
	j.storeResponseCookiesAt(rawURL, setCookies, j.now())
}

func (j *CookieJar) storeResponseCookiesAt(rawURL string, setCookies []string, now time.Time) {
	if j == nil || len(setCookies) == 0 {
		return
	}
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return
	}
	reqHost := normalizeCookieHost(parsedURL.Hostname())
	if reqHost == "" {
		return
	}
	reqPath := parsedURL.EscapedPath()
	if reqPath == "" {
		reqPath = "/"
	}
	secureOrigin := strings.EqualFold(parsedURL.Scheme, "https")
	for _, header := range setCookies {
		j.storeOneAt(reqHost, reqPath, secureOrigin, header, now)
	}
}

func (j *CookieJar) storeOneAt(reqHost, reqPath string, secureOrigin bool, header string, now time.Time) {
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return
	}
	nameVal := parts[0]
	eq := strings.IndexByte(nameVal, '=')
	if eq <= 0 {
		return
	}
	name := strings.TrimSpace(nameVal[:eq])
	value := strings.TrimSpace(nameVal[eq+1:])
	if name == "" {
		return
	}

	cookie := Cookie{
		Name:     name,
		Value:    value,
		Domain:   reqHost,
		HostOnly: true,
		Path:     defaultCookiePath(reqPath),
	}

	var hasMaxAge bool
	var maxAgeSecs int64
	var hasExpires bool
	var expiresAt time.Time

	for _, rawAttr := range parts[1:] {
		attr := strings.TrimSpace(rawAttr)
		if attr == "" {
			continue
		}
		attrName := attr
		attrVal := ""
		if idx := strings.IndexByte(attr, '='); idx >= 0 {
			attrName = strings.TrimSpace(attr[:idx])
			attrVal = strings.TrimSpace(attr[idx+1:])
		}
		switch strings.ToLower(attrName) {
		case "domain":
			domain := normalizeCookieHost(strings.TrimPrefix(attrVal, "."))
			if domain == "" {
				continue
			}
			if !domainMatches(reqHost, domain) {
				return
			}
			if suffix, _ := publicsuffix.PublicSuffix(domain); suffix == domain {
				if reqHost != domain {
					return
				}
				cookie.Domain = reqHost
				cookie.HostOnly = true
				continue
			}
			cookie.Domain = domain
			cookie.HostOnly = false
		case "path":
			if strings.HasPrefix(attrVal, "/") {
				cookie.Path = attrVal
			} else {
				cookie.Path = defaultCookiePath(reqPath)
			}
		case "secure":
			if !secureOrigin {
				return
			}
			cookie.Secure = true
		case "httponly":
			cookie.HTTPOnly = true
		case "max-age":
			parsed, err := parseCookieMaxAge(attrVal)
			if err == nil {
				hasMaxAge = true
				maxAgeSecs = parsed
			}
		case "expires":
			if parsed, ok := parseCookieExpires(attrVal); ok {
				hasExpires = true
				expiresAt = parsed
			}
		}
	}

	if hasMaxAge {
		cookie.HasExpiry = true
		cookie.Persistent = true
		if maxAgeSecs <= 0 {
			cookie.ExpiresAt = time.Unix(0, 0).UTC()
		} else {
			if maxAgeSecs > maxCookieDurationSecs {
				maxAgeSecs = maxCookieDurationSecs
			}
			cookie.ExpiresAt = now.Add(time.Duration(maxAgeSecs) * time.Second)
		}
	} else if hasExpires {
		cookie.HasExpiry = true
		cookie.Persistent = true
		cookie.ExpiresAt = expiresAt
	}

	if cookie.HasExpiry && !cookie.ExpiresAt.After(now) {
		j.deleteCookie(cookie.Name, cookie.Domain, cookie.Path)
		return
	}

	j.upsertCookie(cookie)
}

func (j *CookieJar) deleteCookie(name, domain, path string) {
	kept := j.cookies[:0]
	for _, c := range j.cookies {
		if c.Name == name && c.Domain == domain && c.Path == path {
			continue
		}
		kept = append(kept, c)
	}
	j.cookies = kept
}

func (j *CookieJar) upsertCookie(cookie Cookie) {
	for i := range j.cookies {
		if j.cookies[i].Name == cookie.Name &&
			j.cookies[i].Domain == cookie.Domain &&
			j.cookies[i].Path == cookie.Path {
			cookie.createdSeq = j.cookies[i].createdSeq
			j.cookies[i] = cookie
			return
		}
	}
	j.seq++
	cookie.createdSeq = j.seq
	j.cookies = append(j.cookies, cookie)
}

// CookiesForURL returns all non-expired cookies matching rawURL, ordered per
// RFC 6265 (longest path first, then creation order).
func (j *CookieJar) CookiesForURL(rawURL string) []Cookie {
	return j.cookiesForURLAt(rawURL, j.now())
}

func (j *CookieJar) cookiesForURLAt(rawURL string, now time.Time) []Cookie {
	if j == nil || len(j.cookies) == 0 {
		return nil
	}
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil
	}
	scheme := strings.ToLower(strings.TrimSpace(parsedURL.Scheme))
	reqHost := normalizeCookieHost(parsedURL.Hostname())
	if reqHost == "" {
		return nil
	}
	reqPath := parsedURL.EscapedPath()
	if reqPath == "" {
		reqPath = "/"
	}

	active := j.cookies[:0]
	var matches []Cookie
	for _, c := range j.cookies {
		if c.HasExpiry && !c.ExpiresAt.After(now) {
			continue
		}
		active = append(active, c)

		if c.Secure && scheme != "https" {
			continue
		}
		if c.HostOnly {
			if reqHost != c.Domain {
				continue
			}
		} else if !domainMatches(reqHost, c.Domain) {
			continue
		}
		if !pathMatches(reqPath, c.Path) {
			continue
		}
		matches = append(matches, c)
	}
	j.cookies = active

	sort.SliceStable(matches, func(i, k int) bool {
		if len(matches[i].Path) != len(matches[k].Path) {
			return len(matches[i].Path) > len(matches[k].Path)
		}
		return matches[i].createdSeq < matches[k].createdSeq
	})
	return matches
}

// HeaderValue returns the formatted Cookie request header value for rawURL, or
// "" if no cookies match.
func (j *CookieJar) HeaderValue(rawURL string) string {
	return j.headerValueAt(rawURL, j.now())
}

func (j *CookieJar) headerValueAt(rawURL string, now time.Time) string {
	matches := j.cookiesForURLAt(rawURL, now)
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	for i, c := range matches {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(c.Name)
		b.WriteByte('=')
		b.WriteString(c.Value)
	}
	return b.String()
}

// Get returns the value of the named cookie visible to rawURL, if present.
func (j *CookieJar) Get(rawURL, name string) (string, bool) {
	return j.getAt(rawURL, name, j.now())
}

func (j *CookieJar) getAt(rawURL, name string, now time.Time) (string, bool) {
	for _, c := range j.cookiesForURLAt(rawURL, now) {
		if c.Name == name {
			return c.Value, true
		}
	}
	return "", false
}

func normalizeCookieHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	return host
}

func defaultCookiePath(reqPath string) string {
	if reqPath == "" || !strings.HasPrefix(reqPath, "/") {
		return "/"
	}
	if reqPath == "/" {
		return "/"
	}
	lastSlash := strings.LastIndexByte(reqPath, '/')
	if lastSlash <= 0 {
		return "/"
	}
	return reqPath[:lastSlash]
}

func domainMatches(reqHost, domain string) bool {
	if reqHost == domain {
		return true
	}
	if isIPLiteral(reqHost) || isIPLiteral(domain) {
		return false
	}
	return len(reqHost) > len(domain) && strings.HasSuffix(reqHost, "."+domain)
}

func pathMatches(reqPath, cookiePath string) bool {
	if reqPath == cookiePath {
		return true
	}
	if !strings.HasPrefix(reqPath, cookiePath) {
		return false
	}
	if strings.HasSuffix(cookiePath, "/") {
		return true
	}
	return len(reqPath) > len(cookiePath) && reqPath[len(cookiePath)] == '/'
}

func isIPLiteral(host string) bool {
	if strings.ContainsRune(host, ':') {
		return true
	}
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 3 {
			return false
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func parseCookieMaxAge(raw string) (int64, error) {
	if raw == "" {
		return 0, strconv.ErrSyntax
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) && isSignedDigits(raw) {
			if strings.HasPrefix(raw, "-") {
				return 0, nil
			}
			return maxCookieDurationSecs, nil
		}
		return 0, err
	}
	if parsed > maxCookieDurationSecs {
		return maxCookieDurationSecs, nil
	}
	return parsed, nil
}

func isSignedDigits(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '+' || s[0] == '-' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseCookieExpires implements the RFC 6265 section 5.1.1 cookie-date
// algorithm without pulling regexp into TinyGo plugins.
func parseCookieExpires(value string) (time.Time, bool) {
	var foundTime, foundDay, foundMonth, foundYear bool
	var hour, minute, second, day, year int
	var month time.Month

	for _, token := range strings.FieldsFunc(value, isCookieDateDelimiter) {
		if !foundTime {
			if h, m, s, ok := parseCookieTime(token); ok {
				foundTime = true
				hour, minute, second = h, m, s
				continue
			}
		}
		if !foundDay {
			if v, ok := leadingDigits(token, 1, 2); ok {
				foundDay = true
				day = v
				continue
			}
		}
		if !foundMonth {
			if m, ok := cookieMonth(token); ok {
				foundMonth = true
				month = m
				continue
			}
		}
		if !foundYear {
			if v, ok := leadingDigits(token, 2, 4); ok {
				foundYear = true
				year = v
				continue
			}
		}
	}

	if !foundTime || !foundDay || !foundMonth || !foundYear {
		return time.Time{}, false
	}
	if year >= 70 && year <= 99 {
		year += 1900
	} else if year >= 0 && year <= 69 {
		year += 2000
	}
	if day < 1 || day > 31 || year < 1601 || hour > 23 || minute > 59 || second > 59 {
		return time.Time{}, false
	}
	parsed := time.Date(year, month, day, hour, minute, second, 0, time.UTC)
	if parsed.Day() != day || parsed.Month() != month {
		return time.Time{}, false
	}
	return parsed, true
}

func isCookieDateDelimiter(r rune) bool {
	switch {
	case r == 0x09:
		return true
	case r >= 0x20 && r <= 0x2F:
		return true
	case r >= 0x3B && r <= 0x40:
		return true
	case r >= 0x5B && r <= 0x60:
		return true
	case r >= 0x7B && r <= 0x7E:
		return true
	default:
		return false
	}
}

// leadingDigits matches minDigits*maxDigits DIGIT ( non-digit *OCTET ).
func leadingDigits(token string, minDigits, maxDigits int) (int, bool) {
	n := 0
	for n < len(token) && token[n] >= '0' && token[n] <= '9' {
		n++
	}
	if n < minDigits || n > maxDigits {
		return 0, false
	}
	v, err := strconv.Atoi(token[:n])
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseCookieTime matches 1*2DIGIT ":" 1*2DIGIT ":" 1*2DIGIT ( non-digit *OCTET ).
func parseCookieTime(token string) (int, int, int, bool) {
	var parts [3]int
	rest := token
	for i := range parts {
		n := 0
		for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
			n++
		}
		if n < 1 || n > 2 {
			return 0, 0, 0, false
		}
		parts[i], _ = strconv.Atoi(rest[:n])
		rest = rest[n:]
		if i < 2 {
			if rest == "" || rest[0] != ':' {
				return 0, 0, 0, false
			}
			rest = rest[1:]
		}
	}
	return parts[0], parts[1], parts[2], true
}

func cookieMonth(token string) (time.Month, bool) {
	if len(token) < 3 {
		return 0, false
	}
	switch strings.ToLower(token[:3]) {
	case "jan":
		return time.January, true
	case "feb":
		return time.February, true
	case "mar":
		return time.March, true
	case "apr":
		return time.April, true
	case "may":
		return time.May, true
	case "jun":
		return time.June, true
	case "jul":
		return time.July, true
	case "aug":
		return time.August, true
	case "sep":
		return time.September, true
	case "oct":
		return time.October, true
	case "nov":
		return time.November, true
	case "dec":
		return time.December, true
	default:
		return 0, false
	}
}
