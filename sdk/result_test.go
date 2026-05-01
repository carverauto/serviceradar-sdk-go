package sdk

import (
	"encoding/json"
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

	discovery := NewDeviceDiscovery("ual-network-map").
		WithDevice(DiscoveredDevice{
			Hostname:    "NIAHAP-MDF001-WAP001",
			MAC:         "b4:5d:50:c7:46:6c",
			Serial:      "CNC3HN77NW",
			VendorName:  "Aruba",
			Model:       "325",
			Type:        "access_point",
			Role:        "ap_bridge",
			IsAvailable: &available,
			Location: &DeviceLocation{
				SiteCode: "IAH",
				SiteName: "George Bush Intercontinental Airport",
			},
			Metadata: map[string]any{
				"integration_id": "wifi_map:access_point:CNC3HN77NW",
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
	if device["hostname"] != "NIAHAP-MDF001-WAP001" {
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
