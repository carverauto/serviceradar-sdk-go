//go:build !tinygo

package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRunLocalHostExercisesNormalSDKCalls(t *testing.T) {
	credentials := map[string]string{"username": "local-user", "password": "local-password"}
	handlerCalls := 0

	capture, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{"url":"https://inventory.example.test/devices"}`),
		HTTPHandler: func(_ context.Context, request HTTPRequest) (*HTTPResponse, error) {
			handlerCalls++
			if request.URL != "https://inventory.example.test/devices" {
				t.Fatalf("request URL = %q", request.URL)
			}
			if credentials["username"] != "local-user" || credentials["password"] != "local-password" {
				t.Fatal("trusted local host handler did not retain its credentials")
			}
			return &HTTPResponse{Status: 200, Body: []byte(`{"devices":[]}`)}, nil
		},
	}, func() error {
		var config struct {
			URL string `json:"url"`
		}
		if err := LoadConfig(&config); err != nil {
			return err
		}
		response, err := HTTP.DoContext(context.Background(), HTTPRequest{
			Method:       "GET",
			URL:          config.URL,
			ResponseMode: "envelope",
		})
		if err != nil {
			return err
		}
		if response.Status != 200 || string(response.Body) != `{"devices":[]}` {
			return errors.New("unexpected local HTTP response")
		}
		Log.Info("local collection complete")
		if err := EmitTelemetry(TelemetryBatch{Records: []TelemetryRecord{}}); err != nil {
			return err
		}
		return Execute(func() (*Result, error) { return Ok("local run complete"), nil })
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if handlerCalls != 1 {
		t.Fatalf("HTTP handler calls = %d, want 1", handlerCalls)
	}
	if len(capture.ResultJSON) == 0 || len(capture.TelemetryJSON) != 1 || len(capture.Logs) != 1 {
		t.Fatalf("unexpected local host capture: %#v", capture)
	}
	var result Result
	if err := json.Unmarshal(capture.ResultJSON, &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusOK || result.Summary != "local run complete" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if capture.Logs[0].Message != "local collection complete" {
		t.Fatalf("unexpected log: %#v", capture.Logs)
	}

	var config map[string]any
	if err := LoadConfig(&config); err == nil {
		t.Fatal("native host was not restored after local run")
	}
}

func TestRunLocalHostRedactsHandlerErrorsAtSDKBoundary(t *testing.T) {
	secretError := errors.New("dial local-password@example.test")
	_, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(context.Context, HTTPRequest) (*HTTPResponse, error) {
			return nil, secretError
		},
	}, func() error {
		_, requestErr := HTTP.Get("https://example.test")
		return requestErr
	})
	if err == nil || errors.Is(err, secretError) || err.Error() == secretError.Error() {
		t.Fatalf("local HTTP handler error was not reduced to a host error: %v", err)
	}
}

func TestRunLocalHostHTTPResponseModes(t *testing.T) {
	var modes []string
	handler := func(_ context.Context, request HTTPRequest) (*HTTPResponse, error) {
		modes = append(modes, request.ResponseMode)
		return &HTTPResponse{
			Status:  429,
			Headers: map[string]string{"Retry-After": "30", "Content-Type": "application/json"},
			Body:    []byte(`{}`),
		}, nil
	}
	var envelope, statusBody *HTTPResponse
	_, err := RunLocalHost(LocalHostOptions{ConfigJSON: []byte(`{}`), HTTPHandler: handler}, func() error {
		var err error
		envelope, err = HTTP.Do(HTTPRequest{URL: "https://api.example.com/v1/items", ResponseMode: ResponseModeEnvelope})
		if err != nil {
			return err
		}
		statusBody, err = HTTP.Get("https://api.example.com/v1/items")
		return err
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if len(modes) != 2 || modes[0] != ResponseModeEnvelope || modes[1] != ResponseModeStatusBody {
		t.Fatalf("response modes = %v", modes)
	}
	if envelope.Status != 429 || envelope.Header("content-type") != "application/json" {
		t.Fatalf("envelope response = %#v", envelope)
	}
	if wait, ok := envelope.RetryAfter(); !ok || wait != 30*time.Second {
		t.Fatalf("RetryAfter = (%s, %v)", wait, ok)
	}
	if statusBody.Status != 429 || statusBody.Headers != nil || string(statusBody.Body) != `{}` {
		t.Fatalf("status_body response = %#v", statusBody)
	}
}

func TestRunLocalHostEnvelopeTraceTLSAndFollowRedirects(t *testing.T) {
	var seenFollow *bool
	var resp *HTTPResponse
	_, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(_ context.Context, req HTTPRequest) (*HTTPResponse, error) {
			seenFollow = req.FollowRedirects
			return &HTTPResponse{
				Status:     302,
				Headers:    map[string]string{"Location": "/next"},
				SetCookies: []string{"sid=abc; Path=/; Secure", "csrf=def; Path=/"},
				Body:       []byte(`{"ok":true}`),
				Trace: &HTTPTrace{
					DNSDurationNS:          1000,
					ConnectDurationNS:      2000,
					TLSHandshakeDurationNS: 3000,
					TTFBDurationNS:         4000,
					TransferDurationNS:     500,
					TotalDurationNS:        10500,
					ConnectionReused:       false,
					RedirectCount:          0,
					FinalURL:               "https://sso.example.com/login",
					Protocol:               "HTTP/2.0",
					RemoteAddr:             "192.0.2.10:443",
					Hops: []HTTPHopTrace{
						{URL: "https://sso.example.com/login", Status: 302, TTFBDurationNS: 4000},
					},
				},
				TLS: &HTTPTLSInfo{
					Version:           "TLS1.3",
					CipherSuite:       "TLS_AES_128_GCM_SHA256",
					ALPN:              "h2",
					LeafSubjectCN:     "sso.example.com",
					LeafIssuerCN:      "Test CA",
					LeafNotAfterUnix:  Int64(1790553600),
					LeafDaysRemaining: Int(0),
				},
			}, nil
		},
	}, func() error {
		var reqErr error
		resp, reqErr = HTTP.Do(HTTPRequest{
			Method:          "POST",
			URL:             "https://sso.example.com/login",
			ResponseMode:    ResponseModeEnvelope,
			FollowRedirects: Bool(false),
		})
		return reqErr
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if seenFollow == nil || *seenFollow {
		t.Fatalf("FollowRedirects = %v, want false", seenFollow)
	}
	if resp == nil || resp.Trace == nil || resp.TLS == nil {
		t.Fatalf("expected Trace and TLS in response, got %#v", resp)
	}
	if len(resp.SetCookies) != 2 || resp.TLS.LeafDaysRemaining == nil || *resp.TLS.LeafDaysRemaining != 0 || resp.Trace.TotalDurationNS != 10500 {
		t.Fatalf("unexpected envelope fields: %#v", resp)
	}
}

func TestHTTPClientReuseAcrossSequentialAndConcurrentRequests(t *testing.T) {
	client := &HTTPClient{MaxResponseBytes: 4096}

	var statusResponses []*HTTPResponse
	_, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(_ context.Context, req HTTPRequest) (*HTTPResponse, error) {
			if req.URL == "https://example.test/too-large" {
				return &HTTPResponse{Status: 200, Body: make([]byte, 5000)}, nil
			}
			return &HTTPResponse{Status: 200, Body: []byte("body:" + req.URL)}, nil
		},
	}, func() error {
		// 1. Sequential status_body calls must not mutate prior responses when scratch buffer is reused.
		for i := 0; i < 64; i++ {
			resp, reqErr := client.Get(fmt.Sprintf("https://example.test/step/%d", i))
			if reqErr != nil {
				return reqErr
			}
			statusResponses = append(statusResponses, resp)
		}

		// 2. Host too-large returns HostError(-3) cleanly without breaking subsequent calls.
		_, largeErr := client.Get("https://example.test/too-large")
		var hErr HostError
		if !errors.As(largeErr, &hErr) || hErr.Code != hostErrTooLarge {
			return fmt.Errorf("expected hostErrTooLarge, got %v", largeErr)
		}

		// 3. Concurrent envelope calls under -race must remain isolated.
		var wg sync.WaitGroup
		errCh := make(chan error, 16)
		for g := 0; g < 16; g++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				u := fmt.Sprintf("https://example.test/goroutine/%d", idx)
				resp, gErr := client.DoContext(context.Background(), HTTPRequest{
					Method:       "GET",
					URL:          u,
					ResponseMode: ResponseModeEnvelope,
				})
				if gErr != nil {
					errCh <- gErr
					return
				}
				if string(resp.Body) != "body:"+u {
					errCh <- fmt.Errorf("got body %q, want %q", string(resp.Body), "body:"+u)
				}
			}(g)
		}
		wg.Wait()
		close(errCh)
		for gErr := range errCh {
			return gErr
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if len(statusResponses) != 64 {
		t.Fatalf("len(statusResponses) = %d, want 64", len(statusResponses))
	}
	for i, resp := range statusResponses {
		want := fmt.Sprintf("body:https://example.test/step/%d", i)
		if string(resp.Body) != want {
			t.Fatalf("statusResponses[%d].Body = %q, want %q", i, string(resp.Body), want)
		}
	}
}

func TestRunLocalHostHTTPDoSupportsLargeEnvelopeResponse(t *testing.T) {
	largeBody := bytes.Repeat([]byte("a"), 5*1024*1024)
	var resp *HTTPResponse

	_, err := RunLocalHost(LocalHostOptions{
		ConfigJSON: []byte(`{}`),
		HTTPHandler: func(_ context.Context, req HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{
				Status:  http.StatusOK,
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    largeBody,
			}, nil
		},
	}, func() error {
		var reqErr error
		resp, reqErr = HTTP.Do(HTTPRequest{
			Method:       http.MethodGet,
			URL:          "https://example.test/topology/snapshot",
			ResponseMode: ResponseModeEnvelope,
		})
		return reqErr
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if resp == nil || resp.Status != http.StatusOK {
		t.Fatalf("unexpected response = %#v", resp)
	}
	if !bytes.Equal(resp.Body, largeBody) {
		t.Fatalf("response body len = %d, want %d", len(resp.Body), len(largeBody))
	}
}
