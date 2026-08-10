package sdk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAdvisoryFeedBatchSerializesGenericContract(t *testing.T) {
	batch := NewAdvisoryFeedBatch(
		"com.example.feed",
		NewAdvisorySource("example", "normalized").WithDisplayName("Example Normalized Feed"),
		NewAdvisorySnapshot("vulnerability-feeds/example/sha256.json", strings.Repeat("a", 64)).
			WithContentType("application/json").
			WithSizeBytes(42),
	).WithAdvisory(
		NewAdvisoryRecord("CVE-2026-1").
			WithCVE("CVE-2026-1").
			WithSeverity("high").
			WithCoordinate(
				NewPURLCoordinate("pkg:deb/debian/openssl@3.0.13?arch=amd64").
					WithMatchSemantics("producer_normalized").
					WithVersionRange(map[string]any{"fixed_version": "3.0.14"}),
			).
			WithReference("https://example.test/CVE-2026-1"),
	)

	payload, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("marshal advisory batch: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode advisory batch: %v", err)
	}
	if decoded["schema_version"] != AdvisoryFeedContractVersion {
		t.Fatalf("schema_version = %v", decoded["schema_version"])
	}
	if strings.Contains(strings.ToLower(string(payload)), "vulncheck") ||
		strings.Contains(strings.ToLower(string(payload)), "cisa") ||
		strings.Contains(strings.ToLower(string(payload)), "nvd") {
		t.Fatalf("generic advisory contract leaked built-in provider assumptions: %s", payload)
	}
	if !strings.Contains(string(payload), `"affected_coordinates"`) ||
		!strings.Contains(string(payload), `"object_key"`) {
		t.Fatalf("advisory payload missing required fields: %s", payload)
	}
}

func TestResultSerializesAdvisoryFeedBatch(t *testing.T) {
	batch := NewAdvisoryFeedBatch(
		"com.example.feed",
		NewAdvisorySource("example", "normalized"),
		NewAdvisorySnapshot("vulnerability-feeds/example/sha256.json", strings.Repeat("b", 64)),
	).WithAdvisory(
		NewAdvisoryRecord("CVE-2026-2").WithCoordinate(
			NewCPECoordinate("cpe:2.3:a:example:package:1.0:*:*:*:*:*:*:*"),
		),
	)

	payload, err := Ok("submitted advisory batch").WithAdvisoryFeed(batch).Serialize()
	if err != nil {
		t.Fatalf("serialize result: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	feeds, ok := decoded["advisory_feeds"].([]any)
	if !ok || len(feeds) != 1 {
		t.Fatalf("advisory_feeds = %#v", decoded["advisory_feeds"])
	}
}
