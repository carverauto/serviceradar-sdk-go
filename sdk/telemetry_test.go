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
