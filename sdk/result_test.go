package sdk

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSerializeIncludesMetrics(t *testing.T) {
	warn := 50.0
	crit := 100.0

	res := NewResult()
	res.SetStatus(StatusWarning)
	res.SetSummary("latency high")
	res.AddMetric("latency_ms", 75, "ms", &ThresholdSpec{Warn: &warn, Crit: &crit})
	res.AddStatCard("Latency", "75ms", "warning")
	res.EmitEvent(SeverityWarning, "latency high", "latency_threshold")
	res.RequestImmediateAlert("latency_threshold")

	payload, err := res.Serialize()
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	var decoded map[string]any

	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded["status"] != string(StatusWarning) {
		t.Fatalf("expected status %s", StatusWarning)
	}
	if decoded["summary"] != "latency high" {
		t.Fatalf("expected summary to be set")
	}
	if decoded["alert_hint"] != true {
		t.Fatalf("expected alert_hint true")
	}
	if decoded["condition_id"] != "latency_threshold" {
		t.Fatalf("expected condition_id to be set")
	}

	events, ok := decoded["events"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("expected events to be present")
	}

	event, ok := events[0].(map[string]any)
	if !ok {
		t.Fatalf("expected event map")
	}

	if event["class_uid"] == nil || event["type_uid"] == nil || event["activity_id"] == nil {
		t.Fatalf("expected ocsf fields in event")
	}
}

func TestSerializeIncludesDeviceDiscovery(t *testing.T) {
	available := true

	discovery := NewDeviceDiscovery("example-network-map").
		WithDevice(DiscoveredDevice{
			Hostname:    "SITE01-MDF001-WAP001",
			MAC:         "00:00:5e:00:53:01",
			Serial:      "SN0000000001",
			VendorName:  "ExampleVendor",
			Model:       "325",
			Type:        "access_point",
			Role:        "ap_bridge",
			IsAvailable: &available,
			Location: &DeviceLocation{
				SiteCode: "SITE01",
				SiteName: "Example Regional Airport",
			},
			Metadata: map[string]any{
				"integration_id": "wifi_map:access_point:SN0000000001",
			},
		})

	payload, err := NewResult().
		WithStatus(StatusOK).
		WithSummary("discovered 1 device").
		WithDeviceDiscovery(*discovery).
		Serialize()
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	envelopes, ok := decoded["device_discovery"].([]any)
	if !ok || len(envelopes) != 1 {
		t.Fatalf("expected one device discovery envelope")
	}

	envelope := envelopes[0].(map[string]any)
	if envelope["schema"] != DeviceDiscoverySchemaV1 {
		t.Fatalf("unexpected device discovery schema: %v", envelope["schema"])
	}

	devices := envelope["devices"].([]any)
	device := devices[0].(map[string]any)
	if device["hostname"] != "SITE01-MDF001-WAP001" {
		t.Fatalf("unexpected hostname: %v", device["hostname"])
	}
}

func TestApplyThresholds(t *testing.T) {
	warn := 10.0
	crit := 20.0

	res := NewResult()

	res.ApplyThresholds(5, &warn, &crit)
	if res.Status != StatusOK {
		t.Fatalf("expected OK, got %s", res.Status)
	}

	res = NewResult()

	res.ApplyThresholds(12, &warn, &crit)
	if res.Status != StatusWarning {
		t.Fatalf("expected WARNING, got %s", res.Status)
	}

	res = NewResult()

	res.ApplyThresholds(25, &warn, &crit)
	if res.Status != StatusCritical {
		t.Fatalf("expected CRITICAL, got %s", res.Status)
	}
}

func TestSerializeDefaultsWithoutMutation(t *testing.T) {
	res := &Result{}

	payload, err := res.Serialize()
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	if res.SchemaVersion != 0 {
		t.Fatalf("expected SchemaVersion to remain zero")
	}

	if res.Status != "" {
		t.Fatalf("expected Status to remain empty")
	}

	if res.ObservedAt != "" {
		t.Fatalf("expected ObservedAt to remain empty")
	}

	var decoded map[string]any

	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded["schema_version"] != float64(1) {
		t.Fatalf("expected schema_version to default to 1")
	}

	if decoded["status"] != string(StatusUnknown) {
		t.Fatalf("expected status %s", StatusUnknown)
	}

	if decoded["summary"] != string(StatusUnknown) {
		t.Fatalf("expected summary %s", StatusUnknown)
	}

	if decoded["observed_at"] == "" {
		t.Fatalf("expected observed_at to be set")
	}
}

func TestTargetResultSerializesCheckInstanceIdentity(t *testing.T) {
	ctx := TargetContext{
		UID:               "check-1",
		CheckInstanceID:   "check-1",
		DescriptorID:      "http.url.availability",
		DescriptorVersion: "1.0.0",
		TargetKind:        TargetKindService,
		Target: map[string]any{
			"monitored_service_id": "service-1",
			"device_uid":           "sr:device-1",
		},
	}

	payload, err := TargetResult(ctx, StatusOK, "HTTP 200").Serialize()
	if err != nil {
		t.Fatalf("serialize target result: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal target result: %v", err)
	}

	if decoded["check_instance_id"] != "check-1" {
		t.Fatalf("expected check_instance_id, got %v", decoded["check_instance_id"])
	}
	if decoded["monitored_service_id"] != "service-1" {
		t.Fatalf("expected monitored_service_id, got %v", decoded["monitored_service_id"])
	}
	if decoded["device_uid"] != "sr:device-1" {
		t.Fatalf("expected device_uid, got %v", decoded["device_uid"])
	}
}

func TestServiceMonitoringResultFixture(t *testing.T) {
	raw, err := os.ReadFile("../testdata/service_monitoring_result.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	if decoded["check_instance_id"] != "check-1" {
		t.Fatalf("expected check_instance_id in fixture")
	}
	if decoded["monitored_service_id"] != "service-1" {
		t.Fatalf("expected monitored_service_id in fixture")
	}
}

func TestAttachSignalSchemaRef(t *testing.T) {
	event := NewOCSFEventLogActivity("camera event", SeverityWarning)

	AttachSignalSchemaRef(&event, SignalSchemaRef{
		ProducerID:             "axis-camera",
		ProducerVersion:        "0.1.0",
		SchemaID:               "com.carverauto.axis_camera.event_log",
		SchemaVersion:          "1.0.0",
		DisplayContractID:      "com.carverauto.axis_camera.event_log.display",
		DisplayContractVersion: "1.0.0",
		DisplayContract:        "display/event_log_activity.display.json",
		SignalType:             SignalSchemaSignalTypeEvent,
		PayloadKind:            SignalSchemaPayloadKindOCSFEvent,
	})

	serviceRadar, ok := event.Metadata[SignalSchemaMetadataServiceRadar].(map[string]any)
	if !ok {
		t.Fatalf("expected service_radar metadata, got %#v", event.Metadata[SignalSchemaMetadataServiceRadar])
	}

	ref, ok := serviceRadar[SignalSchemaMetadataSignalSchema].(map[string]any)
	if !ok {
		t.Fatalf("expected signal_schema metadata, got %#v", serviceRadar[SignalSchemaMetadataSignalSchema])
	}

	if got := ref[SignalSchemaMetadataSchemaID]; got != "com.carverauto.axis_camera.event_log" {
		t.Fatalf("schema id = %#v", got)
	}
	if got := ref[SignalSchemaMetadataDisplayContract]; got != "display/event_log_activity.display.json" {
		t.Fatalf("display contract = %#v", got)
	}
}

func TestAttachSignalSchemaRefHandlesNilEvent(t *testing.T) {
	if got := AttachSignalSchemaRef(nil, SignalSchemaRef{SchemaID: "x"}); got != nil {
		t.Fatalf("expected nil event, got %#v", got)
	}
}
