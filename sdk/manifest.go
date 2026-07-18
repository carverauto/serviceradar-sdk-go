package sdk

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	ProcessorOCSFPassthrough    = "ocsf_passthrough"
	ProcessorOTELLogPassthrough = "otel_log_passthrough"
	ProcessorJSONToOCSF         = "json_to_ocsf"
	ProcessorSecurityFinding    = "security_finding"
	ProcessorScanActivity       = "scan_activity"

	ConflictPolicyReject   = "reject"
	ConflictPolicyOverride = "override"
)

// PluginManifest is the plugin.yaml/package metadata shape accepted by core.
type PluginManifest struct {
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Version       string                     `json:"version"`
	Entrypoint    string                     `json:"entrypoint"`
	Runtime       string                     `json:"runtime,omitempty"`
	Capabilities  []string                   `json:"capabilities"`
	Permissions   map[string]any             `json:"permissions,omitempty"`
	Resources     map[string]any             `json:"resources"`
	Outputs       string                     `json:"outputs"`
	SignalSchemas []SignalSchemaContribution `json:"signal_schemas,omitempty"`
}

// SignalSchemaContribution declares a package-owned signal schema/display
// contract and optional EventWriter processor contributions.
type SignalSchemaContribution struct {
	ID                     string                    `json:"id"`
	Version                string                    `json:"version"`
	SignalType             string                    `json:"signal_type"`
	PayloadKind            string                    `json:"payload_kind"`
	PayloadSchema          string                    `json:"payload_schema"`
	DisplayContract        string                    `json:"display_contract"`
	DisplayContractID      string                    `json:"display_contract_id"`
	DisplayContractVersion string                    `json:"display_contract_version"`
	OCSFSchemaVersion      string                    `json:"ocsf_schema_version,omitempty"`
	ClassUID               int                       `json:"class_uid,omitempty"`
	TypeUID                int                       `json:"type_uid,omitempty"`
	EventWriter            []EventWriterContribution `json:"event_writer,omitempty"`
}

// EventWriterContribution declares how core should route and normalize
// telemetry records emitted by this package. The processor id must reference a
// platform-owned processor engine; packages do not upload executable processors.
type EventWriterContribution struct {
	Name              string         `json:"name"`
	StreamName        string         `json:"stream_name,omitempty"`
	Subject           string         `json:"subject"`
	ProcessorID       string         `json:"processor_id"`
	Destination       map[string]any `json:"destination,omitempty"`
	OCSF              map[string]any `json:"ocsf,omitempty"`
	Mapping           map[string]any `json:"mapping,omitempty"`
	DeviceCorrelation map[string]any `json:"device_correlation,omitempty"`
	Limits            map[string]any `json:"limits,omitempty"`
	ConflictPolicy    string         `json:"conflict_policy,omitempty"`
	BatchSize         int            `json:"batch_size,omitempty"`
	BatchTimeout      int            `json:"batch_timeout,omitempty"`
}

func NewSignalSchemaContribution(id, version, signalType, payloadKind string) SignalSchemaContribution {
	return SignalSchemaContribution{
		ID:                     id,
		Version:                version,
		SignalType:             signalType,
		PayloadKind:            payloadKind,
		PayloadSchema:          "schemas/" + shortSchemaName(id) + ".schema.json",
		DisplayContract:        "display/" + shortSchemaName(id) + ".display.json",
		DisplayContractID:      id + ".display",
		DisplayContractVersion: version,
	}
}

func NewEventWriterContribution(name, subject, processorID string) EventWriterContribution {
	return EventWriterContribution{
		Name:           name,
		Subject:        subject,
		ProcessorID:    processorID,
		ConflictPolicy: ConflictPolicyReject,
	}
}

func (s SignalSchemaContribution) WithEventWriter(contributions ...EventWriterContribution) SignalSchemaContribution {
	s.EventWriter = append(s.EventWriter, contributions...)
	return s
}

func (c EventWriterContribution) WithStreamName(streamName string) EventWriterContribution {
	c.StreamName = streamName
	return c
}

func (c EventWriterContribution) WithDestination(key string, value any) EventWriterContribution {
	if c.Destination == nil {
		c.Destination = map[string]any{}
	}
	c.Destination[key] = value
	return c
}

func (c EventWriterContribution) WithOCSF(key string, value any) EventWriterContribution {
	if c.OCSF == nil {
		c.OCSF = map[string]any{}
	}
	c.OCSF[key] = value
	return c
}

func (c EventWriterContribution) WithMapping(key string, value any) EventWriterContribution {
	if c.Mapping == nil {
		c.Mapping = map[string]any{}
	}
	c.Mapping[key] = value
	return c
}

func (c EventWriterContribution) WithDeviceCorrelation(key string, value any) EventWriterContribution {
	if c.DeviceCorrelation == nil {
		c.DeviceCorrelation = map[string]any{}
	}
	c.DeviceCorrelation[key] = value
	return c
}

func (c EventWriterContribution) WithLimit(key string, value any) EventWriterContribution {
	if c.Limits == nil {
		c.Limits = map[string]any{}
	}
	c.Limits[key] = value
	return c
}

func (c EventWriterContribution) WithBatch(size, timeout int) EventWriterContribution {
	c.BatchSize = size
	c.BatchTimeout = timeout
	return c
}

func (m PluginManifest) Serialize() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}

func (m PluginManifest) Validate() error {
	var errors []string
	if strings.TrimSpace(m.ID) == "" {
		errors = append(errors, "id must be set")
	}
	if strings.TrimSpace(m.Entrypoint) == "" {
		errors = append(errors, "entrypoint must be set")
	}
	if strings.TrimSpace(m.Outputs) == "" {
		errors = append(errors, "outputs must be set")
	}
	if len(m.Resources) == 0 {
		errors = append(errors, "resources must be set")
	}
	for i, schema := range m.SignalSchemas {
		errors = append(errors, schema.validate(fmt.Sprintf("signal_schemas[%d]", i))...)
	}
	if len(errors) > 0 {
		return fmt.Errorf("invalid plugin manifest: %s", strings.Join(errors, "; "))
	}
	return nil
}

func (s SignalSchemaContribution) validate(path string) []string {
	var errors []string
	if !validIdentifier(s.ID) {
		errors = append(errors, path+".id must be a valid contribution id")
	}
	if s.SignalType != SignalSchemaSignalTypeEvent && s.SignalType != SignalSchemaSignalTypeLog {
		errors = append(errors, path+".signal_type is unsupported")
	}
	if s.PayloadKind != SignalSchemaPayloadKindOCSFEvent &&
		s.PayloadKind != SignalSchemaPayloadKindOTELLog &&
		s.PayloadKind != SignalSchemaPayloadKindJSON {
		errors = append(errors, path+".payload_kind is unsupported")
	}
	if !validBundleJSONPath(s.PayloadSchema) {
		errors = append(errors, path+".payload_schema must be a relative JSON path")
	}
	if !validBundleJSONPath(s.DisplayContract) {
		errors = append(errors, path+".display_contract must be a relative JSON path")
	}
	for i, contribution := range s.EventWriter {
		errors = append(errors, contribution.validate(fmt.Sprintf("%s.event_writer[%d]", path, i))...)
	}
	return errors
}

func (c EventWriterContribution) validate(path string) []string {
	var errors []string
	if !validIdentifier(c.Name) {
		errors = append(errors, path+".name must be a valid contribution id")
	}
	if !validNATSSubject(c.Subject) {
		errors = append(errors, path+".subject must be a valid NATS subject")
	}
	if !allowedProcessorID(c.ProcessorID) {
		errors = append(errors, path+".processor_id is unsupported")
	}
	if c.ConflictPolicy != "" &&
		c.ConflictPolicy != ConflictPolicyReject &&
		c.ConflictPolicy != ConflictPolicyOverride {
		errors = append(errors, path+".conflict_policy is unsupported")
	}
	return errors
}

func allowedProcessorID(processorID string) bool {
	switch processorID {
	case ProcessorOCSFPassthrough, ProcessorOTELLogPassthrough, ProcessorJSONToOCSF,
		ProcessorSecurityFinding, ProcessorScanActivity:
		return true
	default:
		return false
	}
}

func validIdentifier(value string) bool {
	return regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,159}$`).MatchString(value)
}

func validBundleJSONPath(value string) bool {
	return value != "" &&
		!strings.HasPrefix(value, "/") &&
		!strings.Contains(value, "../") &&
		strings.HasSuffix(value, ".json")
}

func validNATSSubject(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) == 0 {
		return false
	}
	for i, part := range parts {
		switch {
		case part == "":
			return false
		case part == "*":
			continue
		case part == ">":
			return i == len(parts)-1
		case regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(part):
			continue
		default:
			return false
		}
	}
	return true
}

func shortSchemaName(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) == 0 {
		return id
	}
	return parts[len(parts)-1]
}
