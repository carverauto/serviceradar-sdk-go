package sdk

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPluginManifestSerializesSDKGeneratedSecurityFixture(t *testing.T) {
	manifest := securityFixtureManifest()

	payload, err := manifest.Serialize()
	if err != nil {
		t.Fatalf("serialize manifest: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode generated manifest: %v", err)
	}

	expectedRaw, err := os.ReadFile("../testdata/sdk_generated_security_manifest.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var expected map[string]any
	if err := json.Unmarshal(expectedRaw, &expected); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
		t.Fatalf("generated manifest mismatch\nwant:\n%s\n\ngot:\n%s", expectedJSON, gotJSON)
	}
}

func TestPluginManifestRejectsUnsafeProcessorContribution(t *testing.T) {
	manifest := securityFixtureManifest()
	manifest.SignalSchemas[0].EventWriter[0].ProcessorID = "arbitrary_code"

	err := manifest.Validate()
	if err == nil {
		t.Fatal("expected unsafe processor to fail validation")
	}
	if !strings.Contains(err.Error(), "processor_id") {
		t.Fatalf("expected processor_id error, got %v", err)
	}
}

func securityFixtureManifest() PluginManifest {
	schema := NewSignalSchemaContribution(
		"com.carverauto.security.scan_activity",
		"1.0.0",
		SignalSchemaSignalTypeEvent,
		SignalSchemaPayloadKindJSON,
	)
	schema.OCSFSchemaVersion = "1.9.0-dev"
	schema.ClassUID = 6007
	schema.TypeUID = 600701

	processor := NewEventWriterContribution(
		"security_scan_activity",
		"plugins.security_sample.scan_activity",
		ProcessorScanActivity,
	).
		WithStreamName("events").
		WithDestination("table", "ocsf_events").
		WithOCSF("schema_version", "1.9.0-dev").
		WithOCSF("class_uid", 6007).
		WithOCSF("activity_id", 1).
		WithMapping("fields", map[string]any{
			"device.name": map[string]any{"path": "host.hostname"},
			"message": map[string]any{
				"template": "Security scan completed for {{host.hostname}}",
			},
		}).
		WithDeviceCorrelation("candidates", []string{
			"host.hostname",
			"metadata.service_radar.agent_id",
		}).
		WithLimit("max_output_bytes", 131072).
		WithBatch(25, 250)

	return PluginManifest{
		ID:         "security-sample",
		Name:       "Security Sample",
		Version:    "1.0.0",
		Entrypoint: "run_check",
		Runtime:    "wasi-preview1",
		Capabilities: []string{
			"get_config",
			"log",
			"submit_result",
			"emit_telemetry",
		},
		Permissions: map[string]any{
			"allowed_domains": []any{},
			"allowed_ports":   []any{},
		},
		Resources: map[string]any{
			"requested_memory_mb":  float64(32),
			"requested_cpu_ms":     float64(5000),
			"max_open_connections": float64(4),
		},
		Outputs:       "serviceradar.plugin_result.v1",
		SignalSchemas: []SignalSchemaContribution{schema.WithEventWriter(processor)},
	}
}
