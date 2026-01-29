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
	res.AddMetric("latency_ms", 75, "ms", &Thresholds{Warn: &warn, Crit: &crit})
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
