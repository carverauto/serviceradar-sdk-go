package sdk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResultSerializesAdvisoryFeedBatch(t *testing.T) {
	batch := NewAdvisoryFeedBatch(
		"com.example.vuln-feed",
		AdvisorySource{
			Provider:    "example",
			FeedKey:     "normalized",
			DisplayName: "Example Advisory Feed",
			Enabled:     true,
			Metadata: map[string]any{
				"plugin_id": "example-feed",
			},
		},
		AdvisorySnapshot{
			ObjectKey: "vulnerability-feeds/example/latest.json",
			SHA256:    strings.Repeat("a", 64),
			Accepted:  true,
			Status:    "accepted",
		},
	).WithAdvisory(AdvisoryRecord{
		SourceObjectID: "example:CVE-2026-1",
		AdvisoryID:     "CVE-2026-1",
		CVEID:          "CVE-2026-1",
		Severity:       "high",
		AffectedCoordinates: []AffectedCoordinate{
			{
				Type:           CoordinateTypePURL,
				Value:          "pkg:generic/example/pkg@1.0.0",
				MatchSemantics: "plugin_normalized",
			},
		},
	})

	payload, err := Ok("accepted advisory batch").
		WithAdvisoryFeed(batch).
		Serialize()
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("result payload is invalid JSON: %v", err)
	}

	feeds := decoded["advisory_feed"].([]any)
	feed := feeds[0].(map[string]any)
	if feed["schema_version"] != AdvisoryFeedContractVersion {
		t.Fatalf("schema_version = %v, want %s", feed["schema_version"], AdvisoryFeedContractVersion)
	}

	source := feed["source"].(map[string]any)
	if source["provider"] != "example" || source["feed_key"] != "normalized" {
		t.Fatalf("source = %#v", source)
	}

	labels := decoded["labels"].(map[string]any)
	if labels["advisory_feed_schema"] != AdvisoryFeedContractVersion {
		t.Fatalf("labels = %#v", labels)
	}
}
