package sdk

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestTelemetryBatchSerializesOCSFRecordWithSignalSchemaRef(t *testing.T) {
	event := NewOCSFEventLogActivity("camera motion", SeverityWarning)
	record := NewOCSFTelemetryRecord(event).WithSignalSchemaRef(SignalSchemaRef{
		ProducerID:             "axis",
		ProducerVersion:        "0.1.0",
		SchemaID:               "com.carverauto.axis_camera.event_log",
		SchemaVersion:          "1.0.0",
		DisplayContractID:      "com.carverauto.axis_camera.event_log.display",
		DisplayContractVersion: "1.0.0",
		DisplayContract:        "display/event_log_activity.display.json",
		SignalType:             SignalSchemaSignalTypeEvent,
		PayloadKind:            SignalSchemaPayloadKindOCSFEvent,
	})

	payload, err := (TelemetryBatch{
		Source:  TelemetrySource{SourceType: "axis-camera", SourceInstance: "front-door"},
		Records: []TelemetryRecord{record},
	}).Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("telemetry payload is invalid JSON: %v", err)
	}

	records := decoded["records"].([]any)
	got := records[0].(map[string]any)
	if got["payload_kind"] != SignalSchemaPayloadKindOCSFEvent {
		t.Fatalf("payload_kind = %v, want %s", got["payload_kind"], SignalSchemaPayloadKindOCSFEvent)
	}

	metadata := got["metadata"].(map[string]any)
	if metadata["serviceradar.signal_schema.schema_id"] != "com.carverauto.axis_camera.event_log" {
		t.Fatalf("schema metadata = %#v", metadata)
	}

	eventPayload := got["payload"].(map[string]any)
	if eventPayload["message"] != "camera motion" {
		t.Fatalf("event payload message = %v, want camera motion", eventPayload["message"])
	}
}

func TestEmitTelemetryUsesHostError(t *testing.T) {
	err := EmitTelemetry(TelemetryBatch{
		Records: []TelemetryRecord{
			NewOTELLogTelemetryRecord("log-1", map[string]any{"body": "hello"}),
		},
	})
	if err == nil {
		t.Fatal("EmitTelemetry() expected host error outside TinyGo runtime")
	}
	if hostErr, ok := err.(HostError); !ok || hostErr.Op != "emit_telemetry" {
		t.Fatalf("EmitTelemetry() error = %#v, want HostError emit_telemetry", err)
	}
}

func TestMetricTelemetryRecordSerializesBase64ProtobufPayload(t *testing.T) {
	protoPayload := []byte{0x0a, 0x16, 's', 'e', 'r', 'v', 'i', 'c', 'e', 'r', 'a', 'd', 'a', 'r'}
	record := NewServiceRadarMetricTelemetryRecord("metric-1", protoPayload)

	payload, err := (TelemetryBatch{
		Source:  TelemetrySource{SourceType: "env-sensor", SourceInstance: "rack-a"},
		Records: []TelemetryRecord{record},
	}).Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("telemetry payload is invalid JSON: %v", err)
	}

	records := decoded["records"].([]any)
	got := records[0].(map[string]any)
	if got["payload_kind"] != SignalSchemaPayloadKindServiceRadarMetrics {
		t.Fatalf("payload_kind = %v, want %s", got["payload_kind"], SignalSchemaPayloadKindServiceRadarMetrics)
	}

	encoded, ok := got["payload"].(string)
	if !ok {
		t.Fatalf("payload = %#v, want base64 string", got["payload"])
	}
	decodedPayload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("payload is not valid base64: %v", err)
	}
	if string(decodedPayload) != string(protoPayload) {
		t.Fatalf("payload = %x, want %x", decodedPayload, protoPayload)
	}
}

func TestExternalTopologyV1TelemetryRecordSerializesSnapshot(t *testing.T) {
	if SignalSchemaPayloadKindExternalTopologyV1 != "external_topology_v1" {
		t.Fatalf("SignalSchemaPayloadKindExternalTopologyV1 = %q, want external_topology_v1", SignalSchemaPayloadKindExternalTopologyV1)
	}

	snapshot := map[string]any{
		"schema_version": 1,
		"nodes":          []any{map[string]any{"id": "ext.asn.64512"}},
		"edges":          []any{},
	}
	record := NewExternalTopologyV1TelemetryRecord("topology-1", snapshot)
	if record.ObservedTimeUnixNano <= 0 || record.EventTimeUnixNano <= 0 {
		t.Fatalf("expected positive timestamps, got observed=%d event=%d", record.ObservedTimeUnixNano, record.EventTimeUnixNano)
	}
	if record.ObservedTimeUnixNano != record.EventTimeUnixNano {
		t.Fatalf("observed=%d != event=%d", record.ObservedTimeUnixNano, record.EventTimeUnixNano)
	}
	if record.PayloadKind != SignalSchemaPayloadKindExternalTopologyV1 {
		t.Fatalf("PayloadKind = %q, want %q", record.PayloadKind, SignalSchemaPayloadKindExternalTopologyV1)
	}
	if record.Metadata != nil {
		t.Fatalf("Metadata = %#v, want nil", record.Metadata)
	}

	payload, err := (TelemetryBatch{
		Source:  TelemetrySource{SourceType: "topology-sync", SourceInstance: "primary"},
		Records: []TelemetryRecord{record},
	}).Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("telemetry payload is invalid JSON: %v", err)
	}

	records := decoded["records"].([]any)
	got := records[0].(map[string]any)
	if got["event_id"] != "topology-1" {
		t.Fatalf("event_id = %v, want topology-1", got["event_id"])
	}
	if got["payload_kind"] != SignalSchemaPayloadKindExternalTopologyV1 {
		t.Fatalf("payload_kind = %v, want %s", got["payload_kind"], SignalSchemaPayloadKindExternalTopologyV1)
	}
	if _, hasMetadata := got["metadata"]; hasMetadata {
		t.Fatalf("metadata should be omitted when nil, got %#v", got["metadata"])
	}

	gotPayload := got["payload"].(map[string]any)
	if gotPayload["schema_version"] != float64(1) {
		t.Fatalf("payload.schema_version = %v, want 1", gotPayload["schema_version"])
	}
	gotNodes := gotPayload["nodes"].([]any)
	if len(gotNodes) != 1 || gotNodes[0].(map[string]any)["id"] != "ext.asn.64512" {
		t.Fatalf("payload.nodes = %#v", gotNodes)
	}
	gotEdges := gotPayload["edges"].([]any)
	if len(gotEdges) != 0 {
		t.Fatalf("payload.edges = %#v, want empty", gotEdges)
	}

	emptyIDPayload, err := (TelemetryBatch{
		Records: []TelemetryRecord{NewExternalTopologyV1TelemetryRecord("", snapshot)},
	}).Serialize()
	if err != nil {
		t.Fatalf("Serialize(empty eventID) error = %v", err)
	}
	var decodedEmpty map[string]any
	if err := json.Unmarshal(emptyIDPayload, &decodedEmpty); err != nil {
		t.Fatalf("empty eventID payload is invalid JSON: %v", err)
	}
	gotEmpty := decodedEmpty["records"].([]any)[0].(map[string]any)
	if _, hasEventID := gotEmpty["event_id"]; hasEventID {
		t.Fatalf("event_id should be omitted when empty, got %#v", gotEmpty["event_id"])
	}

	capture, err := RunLocalHost(LocalHostOptions{ConfigJSON: []byte(`{}`)}, func() error {
		return EmitTelemetry(TelemetryBatch{
			Source:  TelemetrySource{SourceType: "topology-sync", SourceInstance: "local"},
			Records: []TelemetryRecord{record},
		})
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if len(capture.TelemetryJSON) != 1 {
		t.Fatalf("len(TelemetryJSON) = %d, want 1", len(capture.TelemetryJSON))
	}
	var captured map[string]any
	if err := json.Unmarshal(capture.TelemetryJSON[0], &captured); err != nil {
		t.Fatalf("captured telemetry is invalid JSON: %v", err)
	}
	capturedRec := captured["records"].([]any)[0].(map[string]any)
	if capturedRec["payload_kind"] != SignalSchemaPayloadKindExternalTopologyV1 || capturedRec["event_id"] != "topology-1" {
		t.Fatalf("captured record = %#v", capturedRec)
	}
}
