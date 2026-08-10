package sdk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProducerScheduleContractSerializesManifestShape(t *testing.T) {
	contract := NewProducerScheduleContract("daily_advisory_refresh", "Refresh advisory feed", "advisory.refresh").
		WithDescription("Downloads and emits normalized advisory batches").
		WithCadence(86_400, 3_600, 2_592_000).
		WithJitterSeconds(120).
		WithSettingsSchema(map[string]any{"type": "object"}).
		WithCredentialRequirements(map[string]any{"refs": []string{"feed_api_token"}}).
		WithPayloadTemplate(map[string]any{"feed_key": "primary"}).
		WithRedaction(map[string]any{"credential_refs": true}).
		WithTimeoutSeconds(600)

	payload, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("marshal producer schedule: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode producer schedule: %v", err)
	}
	if decoded["schedule_id"] != "daily_advisory_refresh" {
		t.Fatalf("schedule_id = %v", decoded["schedule_id"])
	}
	if decoded["command_type"] != ProducerScheduleCommandPluginRunAction {
		t.Fatalf("command_type = %v", decoded["command_type"])
	}
	if decoded["dispatch_scope"] != ProducerScheduleDispatchAssignment {
		t.Fatalf("dispatch_scope = %v", decoded["dispatch_scope"])
	}
	encoded := strings.ToLower(string(payload))
	for _, provider := range []string{"cisa", "nvd", "vulncheck", "osv", "trivy", "scalibr"} {
		if strings.Contains(encoded, provider) {
			t.Fatalf("schedule contract leaked provider-specific assumptions: %s", payload)
		}
	}
	if decoded["jitter_seconds"] != float64(120) {
		t.Fatalf("jitter_seconds = %v", decoded["jitter_seconds"])
	}
	if !strings.Contains(string(payload), `"credential_requirements"`) {
		t.Fatalf("schedule contract missing credential requirements: %s", payload)
	}
}
