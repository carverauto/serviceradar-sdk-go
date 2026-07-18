package sdk

import (
	"encoding/json"
	"testing"
)

func TestArtifactABIShapesSerialize(t *testing.T) {
	openPayload, err := json.Marshal(ArtifactOpenRequest{
		ObjectKey:   "vulnerability-feeds/example/latest.zip",
		Type:        "advisory-feed-snapshot",
		ContentType: "application/zip",
		SHA256:      "abc123",
		SizeBytes:   42,
		Metadata:    map[string]any{"provider": "example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(openPayload) == "{}" || !json.Valid(openPayload) {
		t.Fatalf("invalid open payload: %s", openPayload)
	}

	responsePayload, err := json.Marshal(ArtifactCommitResponse{
		ObjectKey:      "vulnerability-feeds/example/latest.zip",
		SHA256:         "abc123",
		SizeBytes:      42,
		StorageBackend: "agent-gateway",
		Accepted:       true,
		Status:         "accepted",
	})
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(responsePayload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["storage_backend"] != "agent-gateway" || decoded["accepted"] != true {
		t.Fatalf("unexpected commit response payload: %#v", decoded)
	}
}

func TestArtifactStreamWithoutHostReturnsHostError(t *testing.T) {
	_, err := OpenArtifactStream(ArtifactOpenRequest{ObjectKey: "objects/example"})
	if err == nil {
		t.Fatal("OpenArtifactStream() expected host error outside TinyGo runtime")
	}
	if hostErr, ok := err.(HostError); !ok || hostErr.Op != "artifact_open" {
		t.Fatalf("OpenArtifactStream() error = %#v, want HostError artifact_open", err)
	}
}
