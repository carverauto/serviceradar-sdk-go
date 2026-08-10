package sdk

import (
	"encoding/base64"
	"encoding/binary"
	"math"
	"testing"
)

func TestMarshalServiceRadarMetricBatchEncodesCanonicalScalarEnvelope(t *testing.T) {
	payload := MarshalServiceRadarMetricBatch(MetricBatch{
		Resource: MetricResource{
			AgentID:     "agent-1",
			ServiceName: "axis-camera",
			ServiceType: "wasm-plugin",
			Attributes:  []MetricStringMapEntry{{Key: "camera_host", Value: "cam-1"}},
		},
		IngestIdentity: MetricIngestIdentity{
			Source:       "plugin-metrics",
			ProducerID:   "axis-camera",
			ProducerKind: "wasm-plugin",
		},
		Metrics: []Metric{
			{
				Name:       "axis.endpoint_success_total",
				MetricType: "plugin",
				Kind:       MetricKindGauge,
				Unit:       "count",
				Points: []MetricPoint{{
					Value:              3,
					RawValue:           "3",
					RawValueType:       MetricValueTypeDouble,
					ObservedAtUnixNano: 123,
					Attributes:         []MetricStringMapEntry{{Key: "endpoint", Value: "basicdeviceinfo"}},
				}},
			},
			{
				Name:         "axis.uptime_seconds",
				MetricType:   "plugin",
				Kind:         MetricKindSum,
				Temporality:  MetricTemporalityCumulative,
				IsMonotonic:  true,
				Unit:         "s",
				CounterWidth: 64,
				Points: []MetricPoint{{
					Value:              3600,
					RawValue:           "3600",
					RawValueType:       MetricValueTypeUint64,
					ObservedAtUnixNano: 123,
				}},
			},
		},
	})

	root := parseProtoFields(t, payload)
	if got := protoString(t, root, 1); got != MetricEnvelopeSchemaVersion {
		t.Fatalf("schema_version = %q", got)
	}

	resource := parseSingleMessage(t, root, 2)
	if got := protoString(t, resource, 1); got != "agent-1" {
		t.Fatalf("resource.agent_id = %q", got)
	}
	if got := protoString(t, resource, 4); got != "axis-camera" {
		t.Fatalf("resource.service_name = %q", got)
	}
	resourceAttr := parseSingleMessage(t, resource, 20)
	if got := protoString(t, resourceAttr, 1); got != "camera_host" {
		t.Fatalf("resource attribute key = %q", got)
	}

	identity := parseSingleMessage(t, root, 3)
	if got := protoString(t, identity, 1); got != "plugin-metrics" {
		t.Fatalf("ingest_identity.source = %q", got)
	}
	if got := protoString(t, identity, 2); got != MetricEnvelopeSchemaVersion {
		t.Fatalf("ingest_identity.payload_kind = %q", got)
	}

	metrics := root[20]
	if len(metrics) != 2 {
		t.Fatalf("metric count = %d, want 2", len(metrics))
	}
	gauge := parseProtoFields(t, metrics[0].data)
	if got := protoString(t, gauge, 1); got != "axis.endpoint_success_total" {
		t.Fatalf("gauge name = %q", got)
	}
	if got := protoVarint(t, gauge, 3); got != uint64(MetricKindGauge) {
		t.Fatalf("gauge kind = %d", got)
	}
	gaugePoint := parseSingleMessage(t, gauge, 20)
	if got := protoDouble(t, gaugePoint, 1); got != 3 {
		t.Fatalf("gauge point value = %v", got)
	}
	if got := protoString(t, gaugePoint, 2); got != "3" {
		t.Fatalf("gauge raw_value = %q", got)
	}
	if got := protoVarint(t, gaugePoint, 3); got != uint64(MetricValueTypeDouble) {
		t.Fatalf("gauge raw_value_type = %d", got)
	}

	counter := parseProtoFields(t, metrics[1].data)
	if got := protoVarint(t, counter, 3); got != uint64(MetricKindSum) {
		t.Fatalf("counter kind = %d", got)
	}
	if got := protoVarint(t, counter, 4); got != uint64(MetricTemporalityCumulative) {
		t.Fatalf("counter temporality = %d", got)
	}
	if got := protoVarint(t, counter, 5); got != 1 {
		t.Fatalf("counter is_monotonic = %d", got)
	}
	if got := protoVarint(t, counter, 8); got != 64 {
		t.Fatalf("counter_width = %d", got)
	}
}

func TestMetricTelemetryRecordFromBatchSerializesBase64MetricBatch(t *testing.T) {
	record := NewServiceRadarMetricTelemetryRecordFromBatch("metric-event-1", MetricBatch{
		Metrics: []Metric{{
			Name:       "plugin.camera_total",
			MetricType: "plugin",
			Kind:       MetricKindGauge,
			Points:     []MetricPoint{{Value: 2}},
		}},
	})

	if record.PayloadKind != SignalSchemaPayloadKindServiceRadarMetrics {
		t.Fatalf("payload_kind = %q", record.PayloadKind)
	}
	encoded, ok := record.Payload.(string)
	if !ok {
		t.Fatalf("payload = %#v, want base64 string", record.Payload)
	}
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("payload is not base64: %v", err)
	}
	root := parseProtoFields(t, payload)
	if got := protoString(t, root, 1); got != MetricEnvelopeSchemaVersion {
		t.Fatalf("schema_version = %q", got)
	}
	if len(root[20]) != 1 {
		t.Fatalf("metric count = %d, want 1", len(root[20]))
	}
}

type parsedProtoField struct {
	wire   int
	data   []byte
	varint uint64
	fixed  uint64
}

func parseProtoFields(t *testing.T, payload []byte) map[int][]parsedProtoField {
	t.Helper()
	out := map[int][]parsedProtoField{}
	for len(payload) > 0 {
		tag, n := binary.Uvarint(payload)
		if n <= 0 {
			t.Fatalf("invalid protobuf tag in %x", payload)
		}
		payload = payload[n:]
		field := int(tag >> 3)
		wire := int(tag & 0x7)
		parsed := parsedProtoField{wire: wire}

		switch wire {
		case 0:
			value, n := binary.Uvarint(payload)
			if n <= 0 {
				t.Fatalf("invalid varint for field %d", field)
			}
			parsed.varint = value
			payload = payload[n:]
		case 1:
			if len(payload) < 8 {
				t.Fatalf("short fixed64 for field %d", field)
			}
			parsed.fixed = binary.LittleEndian.Uint64(payload[:8])
			payload = payload[8:]
		case 2:
			size, n := binary.Uvarint(payload)
			if n <= 0 || uint64(len(payload[n:])) < size {
				t.Fatalf("invalid length-delimited field %d", field)
			}
			payload = payload[n:]
			parsed.data = append([]byte(nil), payload[:size]...)
			payload = payload[size:]
		default:
			t.Fatalf("unsupported wire type %d for field %d", wire, field)
		}

		out[field] = append(out[field], parsed)
	}
	return out
}

func parseSingleMessage(t *testing.T, fields map[int][]parsedProtoField, field int) map[int][]parsedProtoField {
	t.Helper()
	values := fields[field]
	if len(values) != 1 {
		t.Fatalf("field %d count = %d, want 1", field, len(values))
	}
	return parseProtoFields(t, values[0].data)
}

func protoString(t *testing.T, fields map[int][]parsedProtoField, field int) string {
	t.Helper()
	values := fields[field]
	if len(values) != 1 || values[0].wire != 2 {
		t.Fatalf("field %d is not one string: %#v", field, values)
	}
	return string(values[0].data)
}

func protoVarint(t *testing.T, fields map[int][]parsedProtoField, field int) uint64 {
	t.Helper()
	values := fields[field]
	if len(values) != 1 || values[0].wire != 0 {
		t.Fatalf("field %d is not one varint: %#v", field, values)
	}
	return values[0].varint
}

func protoDouble(t *testing.T, fields map[int][]parsedProtoField, field int) float64 {
	t.Helper()
	values := fields[field]
	if len(values) != 1 || values[0].wire != 1 {
		t.Fatalf("field %d is not one fixed64: %#v", field, values)
	}
	return math.Float64frombits(values[0].fixed)
}
