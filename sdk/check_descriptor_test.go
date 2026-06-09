package sdk

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCheckDescriptorSerializesTargetScopedContract(t *testing.T) {
	descriptor := NewCheckDescriptor("http.url.availability", "1.0.0", "HTTP URL availability").
		WithTargetKinds(TargetKindService).
		WithServiceKinds("http").
		WithProtocols("http", "https").
		WithRequiredTargetFields("endpoint_url").
		WithRequiredCapabilities("http_request", "submit_result").
		WithCredentialRequirements(map[string]any{"mode": "optional", "purpose": "http_auth"}).
		WithTimeoutBounds(map[string]any{"min_seconds": 1, "max_seconds": 30}).
		WithAllowlistPolicy(map[string]any{"derive_from": []any{"target.host", "target.port", "target.path"}})

	raw, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode descriptor: %v", err)
	}

	if decoded["descriptor_id"] != "http.url.availability" {
		t.Fatalf("unexpected descriptor_id: %v", decoded["descriptor_id"])
	}
	if decoded["result_schema_version"] != ResultSchemaTargetCheckV1 {
		t.Fatalf("unexpected result schema: %v", decoded["result_schema_version"])
	}
}

func TestServiceMonitoringDescriptorFixture(t *testing.T) {
	raw, err := os.ReadFile("../testdata/service_monitoring_descriptor.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var descriptor CheckDescriptor
	if err := json.Unmarshal(raw, &descriptor); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if descriptor.DescriptorID != "http.url.availability" {
		t.Fatalf("unexpected descriptor id: %s", descriptor.DescriptorID)
	}
	if descriptor.ResultSchemaVersion != ResultSchemaTargetCheckV1 {
		t.Fatalf("unexpected result schema: %s", descriptor.ResultSchemaVersion)
	}
}
