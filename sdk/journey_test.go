package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type journeyConformanceDoc struct {
	Schema              string                    `json:"schema"`
	NowUnix             int64                     `json:"now_unix"`
	CookieJarVectors    []cookieJarConformanceVec `json:"cookie_jar_vectors"`
	HTTPEnvelopeVectors []httpEnvelopeConformance `json:"http_envelope_vectors"`
	JourneyScenarios    []journeyConformanceScen  `json:"journey_scenarios"`
}

type cookieJarConformanceVec struct {
	Name  string                    `json:"name"`
	Steps []cookieJarConformanceStp `json:"steps"`
}

type cookieJarConformanceStp struct {
	Action     string   `json:"action"`
	URL        string   `json:"url"`
	SetCookies []string `json:"set_cookies"`
	WantHeader string   `json:"want_header"`
}

type httpEnvelopeConformance struct {
	Name                string             `json:"name"`
	Request             httpRequestPayload `json:"request"`
	WantRequestContains []string           `json:"want_request_contains"`
	ResponseJSON        json.RawMessage    `json:"response_json"`
}

type journeyConformanceScen struct {
	Name          string                   `json:"name"`
	JourneyName   string                   `json:"journey_name"`
	WebPropertyID string                   `json:"web_property_id"`
	DeviceID      string                   `json:"device_id"`
	TargetIP      string                   `json:"target_ip"`
	Steps         []journeyConformanceStep `json:"steps"`
	Expected      journeyConformanceExpect `json:"expected"`
}

type journeyConformanceStep struct {
	Name                  string                     `json:"name"`
	Method                string                     `json:"method"`
	URL                   string                     `json:"url"`
	Headers               map[string]string          `json:"headers"`
	FollowRedirects       *bool                      `json:"follow_redirects"`
	ExpectStatus          []int                      `json:"expect_status"`
	ExpectHeader          *nameValueConformance      `json:"expect_header"`
	ExpectBodyContains    string                     `json:"expect_body_contains"`
	ExpectJSONField       *pointerValueConformance   `json:"expect_json_field"`
	ExpectMaxDurationMS   *thresholdFloatConformance `json:"expect_max_duration_ms"`
	ExpectMinCertDays     *thresholdIntConformance   `json:"expect_min_cert_days"`
	ExtractCookie         *extractConformance        `json:"extract_cookie"`
	ExtractJSONField      *extractJSONConformance    `json:"extract_json_field"`
	ExpectedRequestHeader map[string]string          `json:"expected_request_headers"`
	MockResponse          *journeyMockResponse       `json:"mock_response"`
}

type nameValueConformance struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type pointerValueConformance struct {
	Pointer string `json:"pointer"`
	Value   any    `json:"value"`
}

type thresholdFloatConformance struct {
	Warn float64 `json:"warn"`
	Crit float64 `json:"crit"`
}

type thresholdIntConformance struct {
	Warn int `json:"warn"`
	Crit int `json:"crit"`
}

type extractConformance struct {
	Cookie string `json:"cookie"`
	Var    string `json:"var"`
}

type extractJSONConformance struct {
	Pointer string `json:"pointer"`
	Var     string `json:"var"`
}

type journeyMockResponse struct {
	Status     int               `json:"status"`
	Headers    map[string]string `json:"headers"`
	SetCookies []string          `json:"set_cookies"`
	Body       string            `json:"body"`
	Trace      *HTTPTrace        `json:"trace"`
	TLS        *HTTPTLSInfo      `json:"tls"`
}

type journeyConformanceExpect struct {
	Status              string   `json:"status"`
	HTTPCalls           int      `json:"http_calls"`
	StepsCompleted      int      `json:"steps_completed"`
	StepStatuses        []string `json:"step_statuses"`
	AlertHint           bool     `json:"alert_hint"`
	JourneyAvailable    float64  `json:"journey_available"`
	ForbiddenSubstrings []string `json:"forbidden_substrings"`
	ExpectedMetricNames []string `json:"expected_metric_names"`
}

func TestJourneyConformanceVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "fixtures", "journey_conformance.json"))
	if err != nil {
		t.Fatalf("read journey_conformance.json: %v", err)
	}

	var doc journeyConformanceDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode journey_conformance.json: %v", err)
	}
	if doc.Schema != "serviceradar.journey_conformance.v1" {
		t.Fatalf("unexpected schema: %q", doc.Schema)
	}
	now := time.Unix(doc.NowUnix, 0).UTC()

	for _, vec := range doc.CookieJarVectors {
		t.Run("cookie_jar/"+vec.Name, func(t *testing.T) {
			jar := NewCookieJar()
			jar.nowFn = func() time.Time { return now }
			for idx, step := range vec.Steps {
				switch step.Action {
				case "store":
					jar.StoreResponseCookies(step.URL, step.SetCookies)
				case "query":
					if got := jar.HeaderValue(step.URL); got != step.WantHeader {
						t.Fatalf("step %d query(%s) = %q, want %q", idx, step.URL, got, step.WantHeader)
					}
				default:
					t.Fatalf("unknown cookie jar action %q", step.Action)
				}
			}
		})
	}

	for _, vec := range doc.HTTPEnvelopeVectors {
		t.Run("http_envelope/"+vec.Name, func(t *testing.T) {
			wire := string(marshalHTTPRequestPayload(vec.Request))
			for _, want := range vec.WantRequestContains {
				if !strings.Contains(wire, want) {
					t.Fatalf("wire request %s missing %s", wire, want)
				}
			}
			resp, err := decodeEnvelopeHTTPResponse(vec.ResponseJSON, time.Millisecond)
			if err != nil {
				t.Fatalf("decodeEnvelopeHTTPResponse: %v", err)
			}
			if resp.Status != 302 || len(resp.SetCookies) != 2 {
				t.Fatalf("unexpected decoded response: %#v", resp)
			}
			if resp.Trace == nil || len(resp.Trace.Hops) != 1 || resp.TLS == nil || resp.TLS.LeafDaysRemaining != 0 {
				t.Fatalf("unexpected trace/tls: trace=%#v tls=%#v", resp.Trace, resp.TLS)
			}
		})
	}

	for _, scen := range doc.JourneyScenarios {
		t.Run("journey/"+scen.Name, func(t *testing.T) {
			runJourneyConformanceScenario(t, scen, now)
		})
	}
}

func runJourneyConformanceScenario(t *testing.T, scen journeyConformanceScen, now time.Time) {
	t.Helper()

	j := NewJourney(scen.JourneyName)
	j.nowFn = func() time.Time { return now }
	if scen.WebPropertyID != "" {
		j.WithWebPropertyID(scen.WebPropertyID)
	}
	if scen.DeviceID != "" {
		j.WithDeviceID(scen.DeviceID)
	}
	if scen.TargetIP != "" {
		j.WithTargetDeviceIP(scen.TargetIP)
	}

	for _, s := range scen.Steps {
		step := j.Step(s.Name, s.Method, s.URL)
		if len(s.Headers) > 0 {
			step.WithHeaders(s.Headers)
		}
		if s.FollowRedirects != nil {
			step.WithFollowRedirects(*s.FollowRedirects)
		}
		if len(s.ExpectStatus) > 0 {
			step.ExpectStatus(s.ExpectStatus...)
		}
		if s.ExpectHeader != nil {
			step.ExpectHeader(s.ExpectHeader.Name, s.ExpectHeader.Value)
		}
		if s.ExpectBodyContains != "" {
			step.ExpectBodyContains(s.ExpectBodyContains)
		}
		if s.ExpectJSONField != nil {
			step.ExpectJSONField(s.ExpectJSONField.Pointer, s.ExpectJSONField.Value)
		}
		if s.ExpectMaxDurationMS != nil {
			step.ExpectMaxDuration(s.ExpectMaxDurationMS.Warn, s.ExpectMaxDurationMS.Crit)
		}
		if s.ExpectMinCertDays != nil {
			step.ExpectMinCertDays(s.ExpectMinCertDays.Warn, s.ExpectMinCertDays.Crit)
		}
		if s.ExtractCookie != nil {
			step.ExtractCookie(s.ExtractCookie.Cookie, s.ExtractCookie.Var)
		}
		if s.ExtractJSONField != nil {
			step.ExtractJSONField(s.ExtractJSONField.Pointer, s.ExtractJSONField.Var)
		}
	}

	var calls int
	var report *JourneyReport

	capture, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(_ context.Context, req HTTPRequest) (*HTTPResponse, error) {
			idx := calls
			calls++
			if idx >= len(scen.Steps) || scen.Steps[idx].MockResponse == nil {
				t.Fatalf("unexpected HTTP call %d to %s", idx, req.URL)
			}
			if req.ResponseMode != ResponseModeEnvelope {
				t.Fatalf("step %d response_mode = %q, want envelope", idx, req.ResponseMode)
			}
			for hk, hv := range scen.Steps[idx].ExpectedRequestHeader {
				if req.Headers[hk] != hv {
					t.Fatalf("step %d header %q = %q, want %q", idx, hk, req.Headers[hk], hv)
				}
			}
			mock := scen.Steps[idx].MockResponse
			return &HTTPResponse{
				Status:     mock.Status,
				Headers:    mock.Headers,
				SetCookies: mock.SetCookies,
				Body:       []byte(mock.Body),
				Trace:      mock.Trace,
				TLS:        mock.TLS,
			}, nil
		},
	}, func() error {
		var execErr error
		report, execErr = j.Execute(context.Background())
		if execErr != nil {
			return execErr
		}
		if err := EmitTelemetry(report.TelemetryBatch); err != nil {
			return err
		}
		return Execute(func() (*Result, error) { return report.Result, nil })
	})
	if err != nil {
		t.Fatalf("RunLocalHost: %v", err)
	}
	if calls != scen.Expected.HTTPCalls {
		t.Fatalf("http calls = %d, want %d", calls, scen.Expected.HTTPCalls)
	}
	if string(report.Status) != scen.Expected.Status {
		t.Fatalf("status = %s, want %s (summary=%s)", report.Status, scen.Expected.Status, report.Result.Summary)
	}
	if report.StepsCompleted != scen.Expected.StepsCompleted {
		t.Fatalf("steps_completed = %d, want %d", report.StepsCompleted, scen.Expected.StepsCompleted)
	}
	if len(report.Steps) != len(scen.Expected.StepStatuses) {
		t.Fatalf("step count = %d, want %d", len(report.Steps), len(scen.Expected.StepStatuses))
	}
	for i, wantStatus := range scen.Expected.StepStatuses {
		if string(report.Steps[i].Status) != wantStatus {
			t.Fatalf("step[%d] status = %s, want %s", i, report.Steps[i].Status, wantStatus)
		}
	}
	if report.Result.AlertHint != scen.Expected.AlertHint {
		t.Fatalf("alert_hint = %v, want %v", report.Result.AlertHint, scen.Expected.AlertHint)
	}

	resultStr := string(capture.ResultJSON)
	if len(capture.TelemetryJSON) != 1 {
		t.Fatalf("telemetry batches = %d, want 1", len(capture.TelemetryJSON))
	}
	telemetryStr := string(capture.TelemetryJSON[0])

	var tb TelemetryBatch
	if err := json.Unmarshal(capture.TelemetryJSON[0], &tb); err != nil {
		t.Fatalf("decode TelemetryBatch: %v", err)
	}
	if len(tb.Records) != 1 {
		t.Fatalf("telemetry records = %d, want 1", len(tb.Records))
	}
	payloadB64, ok := tb.Records[0].Payload.(string)
	if !ok {
		t.Fatalf("metric payload is not base64 string: %T", tb.Records[0].Payload)
	}
	protoBytes, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		t.Fatalf("base64 decode metric batch: %v", err)
	}
	protoStr := string(protoBytes)

	for _, secret := range scen.Expected.ForbiddenSubstrings {
		if strings.Contains(resultStr, secret) {
			t.Fatalf("result JSON leaked forbidden substring %q: %s", secret, resultStr)
		}
		if strings.Contains(telemetryStr, secret) {
			t.Fatalf("telemetry JSON leaked forbidden substring %q", secret)
		}
		if strings.Contains(protoStr, secret) {
			t.Fatalf("protobuf MetricBatch leaked forbidden substring %q", secret)
		}
	}

	seenMetrics := map[string]float64{}
	for _, m := range report.MetricBatch.Metrics {
		if m.MetricType != "synthetic" {
			t.Fatalf("metric %s has metric_type=%q, want synthetic", m.Name, m.MetricType)
		}
		if len(m.Points) > 0 {
			seenMetrics[m.Name] = m.Points[0].Value
		}
	}
	for _, wantName := range scen.Expected.ExpectedMetricNames {
		if _, ok := seenMetrics[wantName]; !ok {
			t.Fatalf("missing expected metric %q in %#v", wantName, seenMetrics)
		}
	}
	if seenMetrics["synthetic.journey.available"] != scen.Expected.JourneyAvailable {
		t.Fatalf("synthetic.journey.available = %v, want %v", seenMetrics["synthetic.journey.available"], scen.Expected.JourneyAvailable)
	}
	if scen.WebPropertyID != "" {
		foundResourceAttr := false
		for _, attr := range report.MetricBatch.Resource.Attributes {
			if attr.Key == "web_property_id" && attr.Value == scen.WebPropertyID {
				foundResourceAttr = true
			}
		}
		if !foundResourceAttr {
			t.Fatalf("missing web_property_id in Resource.Attributes: %#v", report.MetricBatch.Resource.Attributes)
		}
	}
}

func TestJourneyJSONPathsMissingVarsAndCookieMerge(t *testing.T) {
	j := NewJourney("edge-cases")
	j.WithTargetContext(TargetContext{
		UID:             "target-1",
		CheckInstanceID: "chk-1",
		DescriptorID:    "desc-1",
		Target: map[string]any{
			"device_uid":           "dev-uid-1",
			"monitored_service_id": "svc-1",
		},
	})
	j.Step("step1", "POST", "https://api.example.com/v1/items?secret=1").
		WithJSONBody(`{"hello":"world"}`).
		WithTimeout(250*time.Millisecond).
		WithInsecureTLS(true).
		WithHeader("Cookie", "override=explicit; extra=yes").
		ExpectStatusRange(200, 299).
		ExpectHeaderContains("Content-Type", "json").
		ExpectJSONField("/data/items/0/slash~1key", "ok").
		ExpectJSONField("data.items[0].id", "42").
		ExpectJSONField("/meta/count", 3).
		ExtractHeader("X-Request-Id", "req_id").
		ExtractJSONField("data.items[0].id", "item_id").
		Step("step2", "GET", "https://api.example.com/v1/items/{{item_id}}").
		WithHeader("X-Correlation", "{{req_id}}").
		ExpectStatus(200)

	var seenCookies []string
	var seenURLs []string
	var report *JourneyReport

	_, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(_ context.Context, req HTTPRequest) (*HTTPResponse, error) {
			seenURLs = append(seenURLs, req.URL)
			seenCookies = append(seenCookies, req.Headers["Cookie"])
			if len(seenURLs) == 1 {
				return &HTTPResponse{
					Status: 201,
					Headers: map[string]string{
						"Content-Type": "application/json; charset=utf-8",
						"X-Request-Id": "req-secret-99",
					},
					SetCookies: []string{"override=jar; Path=/", "jar_only=1; Path=/"},
					Body:       []byte(`{"data":{"items":[{"id":"42","slash/key":"ok"}]},"meta":{"count":3}}`),
				}, nil
			}
			if req.Headers["X-Correlation"] != "req-secret-99" {
				t.Fatalf("missing extracted X-Correlation header: %#v", req.Headers)
			}
			return &HTTPResponse{Status: 200, Body: []byte(`ok`)}, nil
		},
	}, func() error {
		var execErr error
		report, execErr = j.Execute(context.Background())
		return execErr
	})
	if err != nil {
		t.Fatalf("RunLocalHost: %v", err)
	}
	if report.Status != StatusOK {
		t.Fatalf("status = %s, summary = %s", report.Status, report.Result.Summary)
	}
	if seenURLs[1] != "https://api.example.com/v1/items/42" {
		t.Fatalf("step2 URL = %q", seenURLs[1])
	}
	if !strings.Contains(seenCookies[1], "jar_only=1") || !strings.Contains(seenCookies[1], "override=jar") {
		t.Fatalf("step2 cookies = %q", seenCookies[1])
	}
	if report.Result.DeviceUID != "dev-uid-1" || report.Result.MonitoredServiceID != "svc-1" {
		t.Fatalf("target context not propagated: %#v", report.Result)
	}

	// Missing template variable fails before issuing HTTP request.
	missingJourney := NewJourney("missing-var")
	missingJourney.Step("bad", "GET", "https://api.example.com/{{missing_tok}}")
	var missingReport *JourneyReport
	httpCalled := false
	_, err = RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(_ context.Context, _ HTTPRequest) (*HTTPResponse, error) {
			httpCalled = true
			return &HTTPResponse{Status: 200}, nil
		},
	}, func() error {
		var execErr error
		missingReport, execErr = missingJourney.Execute(context.Background())
		return execErr
	})
	if err != nil {
		t.Fatalf("RunLocalHost: %v", err)
	}
	if httpCalled {
		t.Fatal("HTTP request should not be issued when template var is missing")
	}
	if missingReport.Status != StatusCritical || !strings.Contains(missingReport.Result.Summary, "missing_tok") {
		t.Fatalf("unexpected missing var report: %#v", missingReport.Result)
	}
}

func TestTinyGoWASIJourneyBuilds(t *testing.T) {
	tinygoPath, err := exec.LookPath("tinygo")
	if err != nil {
		t.Skip("tinygo not installed in PATH")
	}
	outPath := filepath.Join(t.TempDir(), "http-synthetic-journey.wasm")
	cmd := exec.Command(
		tinygoPath,
		"build",
		"-o", outPath,
		"-target=wasi",
		"-gc=conservative",
		"-scheduler=none",
		"-no-debug",
		"./examples/http-synthetic-journey",
	)
	cmd.Dir = filepath.Join("..")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tinygo build failed: %v\n%s", err, string(output))
	}
	info, err := os.Stat(outPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("compiled wasm missing or empty: err=%v", err)
	}
}
