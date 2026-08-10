//go:build !tinygo

package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
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
