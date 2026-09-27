package sdk

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestParseRunOverridesFixture(t *testing.T) {
	data, err := os.ReadFile("../fixtures/plugin_run_overrides_config.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	overrides, err := ParseRunOverrides(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(overrides) != 2 {
		t.Fatalf("overrides = %d, want 2", len(overrides))
	}

	jam := overrides[0]
	if jam.ID != "fault-jam-7" || jam.Kind != "conveyor_jam" || jam.Target != "conveyor-7" ||
		jam.Params["severity"] != "critical" || jam.Expired {
		t.Fatalf("jam override = %+v", jam)
	}
	at := time.Date(2026, 9, 27, 12, 5, 0, 0, time.UTC)
	if !jam.ActiveAt(at) || jam.ActiveAt(jam.ExpiresAt) {
		t.Fatalf("ActiveAt wrong for %+v", jam)
	}
	if !overrides[1].Expired {
		t.Fatalf("second override should be delivered expired: %+v", overrides[1])
	}

	// The plugin's own config still decodes; the host key is simply ignored.
	var cfg struct {
		Site string `json:"site"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.Site != "campus-a" {
		t.Fatalf("plugin config decode: %+v %v", cfg, err)
	}
}

func TestParseRunOverridesWithoutKeyOrWrongSchema(t *testing.T) {
	if overrides, err := ParseRunOverrides([]byte(`{"site":"a"}`)); err != nil || overrides != nil {
		t.Fatalf("no key: %v %v", overrides, err)
	}
	if _, err := ParseRunOverrides([]byte(`{"_serviceradar_run_overrides":{"schema":"other"}}`)); err == nil {
		t.Fatal("expected an error for an unknown schema")
	}
}

func TestActionResultRunOverridesMatchFixture(t *testing.T) {
	result := ActionSucceeded("Injected conveyor jam").
		SetRunOverride("fault-jam-7", "conveyor_jam", "conveyor-7", 10*time.Minute, map[string]any{"severity": "critical"}).
		EndRunOverride("fault-saturation-2")

	got, err := result.Serialize()
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}
	want, err := os.ReadFile("../fixtures/northbound_action_result_run_overrides.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("decode got: %v", err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("action result mismatch\n got: %s\nwant: %s", got, want)
	}
}

func TestActionDescriptorMaxOverrideDuration(t *testing.T) {
	descriptor := NewActionDescriptor("inject_fault", "Inject fault", ActionScopeDevice).WithMaxOverrideDuration(1800)
	data, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["max_override_duration_seconds"] != float64(1800) {
		t.Fatalf("descriptor = %s", data)
	}
}
