package sdk

import (
	"encoding/json"
	"os"
	"testing"
)

func TestActionDescriptorSerializesManifestShape(t *testing.T) {
	descriptor := NewActionDescriptor("hpna.disable_port", "Disable switch port", ActionScopeInterface).
		WithRequiredContext("device.ip", "interface.name").
		WithInputSchema(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reason": map[string]any{"type": "string"},
			},
		}).
		WithSafety(ActionSafetyDestructive).
		WithConfirmationRequired(true)

	payload, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode descriptor: %v", err)
	}

	if decoded["action_id"] != "hpna.disable_port" {
		t.Fatalf("unexpected action_id: %v", decoded["action_id"])
	}
	if decoded["result_schema_version"] != ActionResultSchemaV1 {
		t.Fatalf("unexpected result schema: %v", decoded["result_schema_version"])
	}
	if decoded["requires_confirmation"] != true {
		t.Fatalf("expected confirmation flag")
	}
}

func TestParseActionConfigExtractsInvocationAndPluginConfig(t *testing.T) {
	payload := []byte(`{
		"api_url": "https://ncm.example",
		"action_invocation": {
			"schema": "serviceradar.northbound_action_invocation.v1",
			"phase": "poll",
			"invocation_id": "inv-1",
			"invocation_target_id": "ivt-1",
			"action_id": "hpna.disable_port",
			"targets": [{
				"kind": "interface",
				"device_uid": "sr:device-1",
				"device_ip": "10.0.0.1",
				"interface_uid": "if-1",
				"if_name": "Gi1/0/1"
			}],
			"input_values": {"reason": "test"},
			"continuation_state": {"external_task_id": "task-123"},
			"external_correlation_id": "task-123",
			"poll_attempt_count": 2
		}
	}`)

	config, err := ParseActionConfig(payload)
	if err != nil {
		t.Fatalf("parse action config: %v", err)
	}

	if config.ActionInvocation.InvocationID != "inv-1" {
		t.Fatalf("unexpected invocation id: %s", config.ActionInvocation.InvocationID)
	}
	if config.ActionInvocation.Phase != "poll" {
		t.Fatalf("unexpected phase: %s", config.ActionInvocation.Phase)
	}
	if config.ActionInvocation.InvocationTargetID != "ivt-1" {
		t.Fatalf("unexpected invocation target id: %s", config.ActionInvocation.InvocationTargetID)
	}
	if config.ActionInvocation.ContinuationState["external_task_id"] != "task-123" {
		t.Fatalf("unexpected continuation state: %v", config.ActionInvocation.ContinuationState)
	}
	if config.ActionInvocation.PollAttemptCount != 2 {
		t.Fatalf("unexpected poll attempts: %d", config.ActionInvocation.PollAttemptCount)
	}
	if len(config.ActionInvocation.Targets) != 1 {
		t.Fatalf("expected one target")
	}
	if config.ActionInvocation.Targets[0].Address() != "10.0.0.1" {
		t.Fatalf("unexpected target address: %s", config.ActionInvocation.Targets[0].Address())
	}

	var pluginConfig struct {
		APIURL string `json:"api_url"`
	}
	if err := config.DecodePluginConfig(&pluginConfig); err != nil {
		t.Fatalf("decode plugin config: %v", err)
	}
	if pluginConfig.APIURL != "https://ncm.example" {
		t.Fatalf("unexpected plugin config: %s", pluginConfig.APIURL)
	}
}

func TestDeferredActionResultSerializesPollFields(t *testing.T) {
	payload, err := ActionDeferred("external task accepted").
		WithCorrelationID("job-123").
		WithContinuationState(map[string]any{"external_task_id": "job-123"}).
		WithNextPollDelay(15).
		WithMaxDuration(300).
		WithTargetResult(ActionTargetResult{
			DeviceUID:             "sr:device-1",
			Status:                ActionStatusDeferred,
			ExternalCorrelationID: "job-123",
			ContinuationState:     map[string]any{"external_task_id": "job-123"},
			NextPollDelaySeconds:  15,
			MaxDurationSeconds:    300,
			Result: map[string]any{
				"message": "queued in external system",
			},
		}).
		Serialize()
	if err != nil {
		t.Fatalf("serialize result: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	if decoded["status"] != string(ActionStatusDeferred) {
		t.Fatalf("status = %v", decoded["status"])
	}
	if decoded["next_poll_delay_seconds"] != float64(15) {
		t.Fatalf("next poll delay = %v", decoded["next_poll_delay_seconds"])
	}
	if decoded["max_duration_seconds"] != float64(300) {
		t.Fatalf("max duration = %v", decoded["max_duration_seconds"])
	}
}

func TestParseActionConfigAcceptsNumericInterfaceStatuses(t *testing.T) {
	payload := []byte(`{
		"action_invocation": {
			"schema": "serviceradar.northbound_action_invocation.v1",
			"invocation_id": "inv-2",
			"action_id": "sample.interface.audit",
			"targets": [{
				"kind": "interface",
				"device_uid": "sr:device-1",
				"device_ip": "10.0.0.1",
				"interface_uid": "if-1",
				"if_name": "Gi1/0/1",
				"if_admin_status": 1,
				"if_oper_status": 2
			}]
		}
	}`)

	config, err := ParseActionConfig(payload)
	if err != nil {
		t.Fatalf("parse action config: %v", err)
	}

	target := config.ActionInvocation.Targets[0]
	if target.IfAdminStatus != "up" {
		t.Fatalf("admin status = %q", target.IfAdminStatus)
	}
	if target.IfOperStatus != "down" {
		t.Fatalf("oper status = %q", target.IfOperStatus)
	}
}

func TestActionResultSerializesHandlerShape(t *testing.T) {
	payload, err := ActionSucceeded("disabled interface").
		WithCorrelationID("job-123").
		WithTargetResult(ActionTargetResult{
			DeviceUID:    "sr:device-1",
			InterfaceUID: "if-1",
			Status:       ActionStatusSucceeded,
			Result: map[string]any{
				"changed": true,
			},
		}).
		Serialize()
	if err != nil {
		t.Fatalf("serialize result: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	if decoded["schema"] != ActionResultSchemaV1 {
		t.Fatalf("unexpected schema: %v", decoded["schema"])
	}
	if decoded["status"] != string(ActionStatusSucceeded) {
		t.Fatalf("unexpected status: %v", decoded["status"])
	}
	if decoded["external_correlation_id"] != "job-123" {
		t.Fatalf("unexpected correlation id")
	}
}

func TestActionFixturesDecode(t *testing.T) {
	descriptorBytes, err := os.ReadFile("../fixtures/northbound_action_descriptor.json")
	if err != nil {
		t.Fatalf("read descriptor fixture: %v", err)
	}
	var descriptor ActionDescriptor
	if err := json.Unmarshal(descriptorBytes, &descriptor); err != nil {
		t.Fatalf("decode descriptor fixture: %v", err)
	}
	if descriptor.ActionID != "hpna.disable_port" {
		t.Fatalf("unexpected descriptor action id: %s", descriptor.ActionID)
	}

	invocationBytes, err := os.ReadFile("../fixtures/northbound_action_invocation.json")
	if err != nil {
		t.Fatalf("read invocation fixture: %v", err)
	}
	configBytes := append([]byte(`{"timeout":"30s","action_invocation":`), invocationBytes...)
	configBytes = append(configBytes, '}')

	config, err := ParseActionConfig(configBytes)
	if err != nil {
		t.Fatalf("decode invocation fixture: %v", err)
	}
	if config.ActionInvocation.Targets[0].InterfaceUID != "if-1" {
		t.Fatalf("unexpected fixture target: %s", config.ActionInvocation.Targets[0].InterfaceUID)
	}

	pollRequestBytes, err := os.ReadFile("../fixtures/northbound_action_poll_request.json")
	if err != nil {
		t.Fatalf("read poll request fixture: %v", err)
	}
	pollConfigBytes := append([]byte(`{"timeout":"30s","action_invocation":`), pollRequestBytes...)
	pollConfigBytes = append(pollConfigBytes, '}')

	pollConfig, err := ParseActionConfig(pollConfigBytes)
	if err != nil {
		t.Fatalf("decode poll request fixture: %v", err)
	}
	if pollConfig.ActionInvocation.Phase != "poll" {
		t.Fatalf("unexpected poll phase: %s", pollConfig.ActionInvocation.Phase)
	}
	if pollConfig.ActionInvocation.ContinuationState["external_task_id"] != "hpna-job-123" {
		t.Fatalf("unexpected continuation state: %v", pollConfig.ActionInvocation.ContinuationState)
	}

	resultBytes, err := os.ReadFile("../fixtures/northbound_action_result.json")
	if err != nil {
		t.Fatalf("read result fixture: %v", err)
	}
	var result ActionResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("decode result fixture: %v", err)
	}
	if result.Status != ActionStatusSucceeded {
		t.Fatalf("unexpected result status: %s", result.Status)
	}

	deferredBytes, err := os.ReadFile("../fixtures/northbound_action_deferred_result.json")
	if err != nil {
		t.Fatalf("read deferred result fixture: %v", err)
	}
	var deferred ActionResult
	if err := json.Unmarshal(deferredBytes, &deferred); err != nil {
		t.Fatalf("decode deferred result fixture: %v", err)
	}
	if deferred.Status != ActionStatusDeferred {
		t.Fatalf("unexpected deferred status: %s", deferred.Status)
	}
	if deferred.Targets[0].NextPollDelaySeconds != 15 {
		t.Fatalf("unexpected deferred next poll delay: %d", deferred.Targets[0].NextPollDelaySeconds)
	}

	pollingBytes, err := os.ReadFile("../fixtures/northbound_action_polling_result.json")
	if err != nil {
		t.Fatalf("read polling result fixture: %v", err)
	}
	var polling ActionResult
	if err := json.Unmarshal(pollingBytes, &polling); err != nil {
		t.Fatalf("decode polling result fixture: %v", err)
	}
	if polling.Status != ActionStatusFetching {
		t.Fatalf("unexpected polling status: %s", polling.Status)
	}
}
