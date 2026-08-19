package main

import (
	"encoding/json"
	"testing"

	"github.com/carverauto/serviceradar-sdk-go/sdk"
)

func TestBuildHTTPResultOmitsRetiredInlineMetrics(t *testing.T) {
	result := buildHTTPResult(200, 150, 100, 200)

	if result.Status != sdk.StatusWarning {
		t.Fatalf("status = %q, want %q", result.Status, sdk.StatusWarning)
	}

	payload, err := result.Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if _, exists := decoded["metrics"]; exists {
		t.Fatalf("result contains retired inline metrics: %s", payload)
	}
}
