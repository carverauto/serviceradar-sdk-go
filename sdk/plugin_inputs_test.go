package sdk

import (
	"errors"
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
