package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// PluginInputsSchemaV1 is the control-plane payload schema for policy-driven plugin inputs.
	PluginInputsSchemaV1 = "serviceradar.plugin_inputs.v1"
)

var (
	errPluginInputsNilPayload         = errors.New("plugin inputs payload is nil")
	errPluginInputsInvalidSchema      = errors.New("plugin inputs payload has invalid schema")
	errPluginInputsMissingPolicyID    = errors.New("plugin inputs payload missing policy_id")
	errPluginInputsInvalidPolicyVer   = errors.New("plugin inputs payload has invalid policy_version")
	errPluginInputsMissingAgentID     = errors.New("plugin inputs payload missing agent_id")
	errPluginInputsMissingGeneratedAt = errors.New("plugin inputs payload missing generated_at")
	errPluginInputsMissingInputs      = errors.New("plugin inputs payload missing inputs")
	errPluginInputsMissingInputName   = errors.New("plugin inputs payload missing input name")
	errPluginInputsMissingInputEntity = errors.New("plugin inputs payload missing input entity")
	errPluginInputsMissingInputQuery  = errors.New("plugin inputs payload missing input query")
	errPluginInputsInvalidChunkIndex  = errors.New("plugin inputs payload has invalid input chunk_index")
	errPluginInputsInvalidChunkTotal  = errors.New("plugin inputs payload has invalid input chunk_total")
	errPluginInputsMissingInputHash   = errors.New("plugin inputs payload missing input chunk_hash")
	errPluginInputsMissingInputItems  = errors.New("plugin inputs payload missing input items")
)

// PluginInputsPayload is the typed schema for policy-driven plugin input params.
type PluginInputsPayload struct {
	Schema        string         `json:"schema"`
	PolicyID      string         `json:"policy_id"`
	PolicyVersion int            `json:"policy_version"`
	AgentID       string         `json:"agent_id"`
	GeneratedAt   string         `json:"generated_at"`
	Template      map[string]any `json:"template,omitempty"`
	Inputs        []PluginInput  `json:"inputs"`
}

// PluginInput is one named input stream (for example: devices or interfaces).
type PluginInput struct {
	Name       string           `json:"name"`
	Entity     string           `json:"entity"`
	Query      string           `json:"query"`
	ChunkIndex int              `json:"chunk_index"`
	ChunkTotal int              `json:"chunk_total"`
	ChunkHash  string           `json:"chunk_hash"`
	Items      []map[string]any `json:"items"`
}

// PluginInputItem is a flattened item with input metadata attached.
type PluginInputItem struct {
	Name       string
	Entity     string
	Query      string
	ChunkIndex int
	ChunkTotal int
	ChunkHash  string
	Item       map[string]any
}

// ParsePluginInputsJSON parses and validates a plugin inputs payload JSON document.
func ParsePluginInputsJSON(data []byte) (*PluginInputsPayload, error) {
	var payload PluginInputsPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode plugin inputs payload: %w", err)
	}

	if err := payload.Validate(); err != nil {
		return nil, err
	}

	return &payload, nil
}

// ParsePluginInputsMap parses and validates a plugin inputs payload object.
func ParsePluginInputsMap(m map[string]any) (*PluginInputsPayload, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode plugin inputs map: %w", err)
	}
	return ParsePluginInputsJSON(raw)
}

// Validate checks required schema/fields for a plugin inputs payload.
func (p *PluginInputsPayload) Validate() error {
	if p == nil {
		return errPluginInputsNilPayload
	}

	if strings.TrimSpace(p.Schema) != PluginInputsSchemaV1 {
		return fmt.Errorf("%w: %q", errPluginInputsInvalidSchema, p.Schema)
	}
	if strings.TrimSpace(p.PolicyID) == "" {
		return errPluginInputsMissingPolicyID
	}
	if p.PolicyVersion < 1 {
		return errPluginInputsInvalidPolicyVer
	}
	if strings.TrimSpace(p.AgentID) == "" {
		return errPluginInputsMissingAgentID
	}
	if strings.TrimSpace(p.GeneratedAt) == "" {
		return errPluginInputsMissingGeneratedAt
	}
	if len(p.Inputs) == 0 {
		return errPluginInputsMissingInputs
	}

	for i, in := range p.Inputs {
		if strings.TrimSpace(in.Name) == "" {
			return fmt.Errorf("%w at inputs[%d].name", errPluginInputsMissingInputName, i)
		}
		if strings.TrimSpace(in.Entity) == "" {
			return fmt.Errorf("%w at inputs[%d].entity", errPluginInputsMissingInputEntity, i)
		}
		if strings.TrimSpace(in.Query) == "" {
			return fmt.Errorf("%w at inputs[%d].query", errPluginInputsMissingInputQuery, i)
		}
		if in.ChunkIndex < 0 {
			return fmt.Errorf("%w at inputs[%d].chunk_index", errPluginInputsInvalidChunkIndex, i)
		}
		if in.ChunkTotal < 1 {
			return fmt.Errorf("%w at inputs[%d].chunk_total", errPluginInputsInvalidChunkTotal, i)
		}
		if strings.TrimSpace(in.ChunkHash) == "" {
			return fmt.Errorf("%w at inputs[%d].chunk_hash", errPluginInputsMissingInputHash, i)
		}
		if len(in.Items) == 0 {
			return fmt.Errorf("%w at inputs[%d].items", errPluginInputsMissingInputItems, i)
		}
	}

	return nil
}

// FlattenItems flattens all input items while preserving deterministic payload order.
func (p *PluginInputsPayload) FlattenItems() []PluginInputItem {
	if p == nil || len(p.Inputs) == 0 {
		return nil
	}

	out := make([]PluginInputItem, 0)
	for _, in := range p.Inputs {
		for _, item := range in.Items {
			out = append(out, PluginInputItem{
				Name:       in.Name,
				Entity:     in.Entity,
				Query:      in.Query,
				ChunkIndex: in.ChunkIndex,
				ChunkTotal: in.ChunkTotal,
				ChunkHash:  in.ChunkHash,
				Item:       item,
			})
		}
	}
	return out
}

// EachItem iterates every item in payload order until callback returns an error.
func (p *PluginInputsPayload) EachItem(fn func(PluginInputItem) error) error {
	if fn == nil {
		return nil
	}
	for _, item := range p.FlattenItems() {
		if err := fn(item); err != nil {
			return err
		}
	}
	return nil
}

// ItemsByEntity returns all flattened items for the specified entity.
func (p *PluginInputsPayload) ItemsByEntity(entity string) []PluginInputItem {
	entity = strings.TrimSpace(strings.ToLower(entity))
	if entity == "" {
		return nil
	}

	out := make([]PluginInputItem, 0)
	for _, item := range p.FlattenItems() {
		if strings.ToLower(strings.TrimSpace(item.Entity)) == entity {
			out = append(out, item)
		}
	}
	return out
}

// ItemsByName returns all flattened items for the specified input name.
func (p *PluginInputsPayload) ItemsByName(name string) []PluginInputItem {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return nil
	}

	out := make([]PluginInputItem, 0)
	for _, item := range p.FlattenItems() {
		if strings.ToLower(strings.TrimSpace(item.Name)) == name {
			out = append(out, item)
		}
	}
	return out
}
