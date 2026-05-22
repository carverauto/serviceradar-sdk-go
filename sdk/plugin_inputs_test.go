package sdk

import (
	"errors"
	"os"
	"strings"
	"testing"
)

var errStopIteration = errors.New("stop iteration")

func TestParsePluginInputsJSON_Valid(t *testing.T) {
	raw := []byte(`{
		"schema":"serviceradar.plugin_inputs.v1",
		"policy_id":"policy-1",
		"policy_version":3,
		"agent_id":"agent-a",
		"generated_at":"2026-02-21T23:00:00Z",
		"inputs":[
			{
				"name":"devices",
				"entity":"devices",
				"query":"in:devices vendor:AXIS",
				"chunk_index":0,
				"chunk_total":1,
				"chunk_hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"items":[
					{"uid":"sr:device:1","ip":"10.0.0.1"},
					{"uid":"sr:device:2","ip":"10.0.0.2"}
				]
			}
		]
	}`)

	p, err := ParsePluginInputsJSON(raw)
	if err != nil {
		t.Fatalf("ParsePluginInputsJSON() error = %v", err)
	}

	if p.PolicyID != "policy-1" {
		t.Fatalf("expected policy_id policy-1, got %q", p.PolicyID)
	}
	if len(p.Inputs) != 1 {
		t.Fatalf("expected 1 input block, got %d", len(p.Inputs))
	}
	if len(p.Inputs[0].Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(p.Inputs[0].Items))
	}
}

func TestParsePluginInputsJSON_InvalidSchema(t *testing.T) {
	raw := []byte(`{
		"schema":"bad.schema",
		"policy_id":"policy-1",
		"policy_version":1,
		"agent_id":"agent-a",
		"generated_at":"2026-02-21T23:00:00Z",
		"inputs":[{"name":"devices","entity":"devices","query":"in:devices","chunk_index":0,"chunk_total":1,"chunk_hash":"h","items":[{"uid":"x"}]}]
	}`)

	_, err := ParsePluginInputsJSON(raw)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "invalid schema") {
		t.Fatalf("expected invalid schema error, got %v", err)
	}
}

func TestPluginInputsPayload_FlattenAndFilters(t *testing.T) {
	payload := &PluginInputsPayload{
		Schema:        PluginInputsSchemaV1,
		PolicyID:      "policy-1",
		PolicyVersion: 1,
		AgentID:       "agent-a",
		GeneratedAt:   "2026-02-21T23:00:00Z",
		Inputs: []PluginInput{
			{
				Name:       "devices",
				Entity:     "devices",
				Query:      "in:devices",
				ChunkIndex: 0,
				ChunkTotal: 1,
				ChunkHash:  "a",
				Items: []map[string]any{
					{"uid": "d1"},
					{"uid": "d2"},
				},
			},
			{
				Name:       "interfaces",
				Entity:     "interfaces",
				Query:      "in:interfaces",
				ChunkIndex: 0,
				ChunkTotal: 1,
				ChunkHash:  "b",
				Items: []map[string]any{
					{"id": "if1"},
				},
			},
		},
	}

	items := payload.FlattenItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 flattened items, got %d", len(items))
	}

	deviceItems := payload.ItemsByEntity("devices")
	if len(deviceItems) != 2 {
		t.Fatalf("expected 2 device items, got %d", len(deviceItems))
	}

	named := payload.ItemsByName("interfaces")
	if len(named) != 1 {
		t.Fatalf("expected 1 interfaces item, got %d", len(named))
	}
}

func TestPluginInputsPayload_EachItemStopsOnError(t *testing.T) {
	payload := &PluginInputsPayload{
		Schema:        PluginInputsSchemaV1,
		PolicyID:      "policy-1",
		PolicyVersion: 1,
		AgentID:       "agent-a",
		GeneratedAt:   "2026-02-21T23:00:00Z",
		Inputs: []PluginInput{
			{
				Name:       "devices",
				Entity:     "devices",
				Query:      "in:devices",
				ChunkIndex: 0,
				ChunkTotal: 1,
				ChunkHash:  "a",
				Items: []map[string]any{
					{"uid": "d1"},
					{"uid": "d2"},
				},
			},
		},
	}

	seen := 0

	err := payload.EachItem(func(item PluginInputItem) error {
		seen++
		if item.Item["uid"] == "d2" {
			return errStopIteration
		}
		return nil
	})

	if !errors.Is(err, errStopIteration) {
		t.Fatalf("expected stop iteration error, got %v", err)
	}
	if seen != 2 {
		t.Fatalf("expected to visit 2 items, visited %d", seen)
	}
}

func TestPluginInputsPayload_TargetContexts(t *testing.T) {
	raw := []byte(`{
		"schema":"serviceradar.plugin_inputs.v1",
		"policy_id":"monitoring_binding:binding-1",
		"policy_version":1,
		"agent_id":"agent-a",
		"generated_at":"2026-05-21T23:00:00Z",
		"inputs":[{
			"name":"monitoring_binding:binding-1",
			"entity":"monitoring_checks",
			"query":"monitoring_binding_id=binding-1",
			"chunk_index":0,
			"chunk_total":1,
			"chunk_hash":"abc",
			"items":[{
				"uid":"check-1",
				"check_instance_id":"check-1",
				"monitoring_binding_id":"binding-1",
				"descriptor_id":"http.url.availability",
				"descriptor_version":"1.0.0",
				"target_kind":"service",
				"target":{
					"monitored_service_id":"service-1",
					"endpoint_url":"https://example.test/health",
					"host":"example.test",
					"port":443,
					"path":"/health",
					"device_uid":"sr:device-1"
				},
				"credential_policy":{
					"credential_brokers":[{
						"grant_id":"grant-1",
						"credential_secret_ref":"credentialref:network-credential-secret:secret-1",
						"grant_type":"http_auth"
					}]
				}
			}]
		}]
	}`)

	payload, err := ParsePluginInputsJSON(raw)
	if err != nil {
		t.Fatalf("parse target payload: %v", err)
	}

	contexts, err := payload.TargetContexts()
	if err != nil {
		t.Fatalf("target contexts: %v", err)
	}

	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}

	ctx := contexts[0]
	if ctx.CheckInstanceID != "check-1" {
		t.Fatalf("unexpected check instance: %s", ctx.CheckInstanceID)
	}
	if ctx.MonitoredServiceID() != "service-1" {
		t.Fatalf("unexpected monitored service: %s", ctx.MonitoredServiceID())
	}
	if ctx.DeviceUID() != "sr:device-1" {
		t.Fatalf("unexpected device uid: %s", ctx.DeviceUID())
	}
	if ctx.EndpointURL() != "https://example.test/health" {
		t.Fatalf("unexpected endpoint url: %s", ctx.EndpointURL())
	}
	if ctx.Port() != 443 {
		t.Fatalf("unexpected port: %d", ctx.Port())
	}
	if len(ctx.CredentialGrants()) != 1 || ctx.CredentialGrants()[0].GrantID != "grant-1" {
		t.Fatalf("expected credential grant")
	}
}

func TestServiceMonitoringInputFixture(t *testing.T) {
	raw, err := os.ReadFile("../testdata/service_monitoring_input.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	payload, err := ParsePluginInputsJSON(raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	contexts, err := payload.TargetContexts()
	if err != nil {
		t.Fatalf("decode fixture target contexts: %v", err)
	}

	if len(contexts) != 1 || contexts[0].CheckInstanceID != "check-1" {
		t.Fatalf("unexpected fixture contexts: %#v", contexts)
	}
}
