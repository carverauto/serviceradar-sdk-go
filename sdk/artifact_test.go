package sdk

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestArtifactOpenRequestSerializesObjectMetadata(t *testing.T) {
	payload, err := json.Marshal(ArtifactOpenRequest{
		ObjectKey:   "feeds/nvd/snapshot.zip",
		ContentType: "application/zip",
		SHA256:      "abc123",
		SizeBytes:   42,
		Attributes: map[string]string{
			"feed": "nvd",
		},
	})
	if err != nil {
		t.Fatalf("marshal artifact request: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode artifact request: %v", err)
	}
	if decoded["object_key"] != "feeds/nvd/snapshot.zip" {
		t.Fatalf("object_key = %v", decoded["object_key"])
	}
	if decoded["content_type"] != "application/zip" {
		t.Fatalf("content_type = %v", decoded["content_type"])
	}
}

func TestOpenArtifactStreamReportsHostErrorOutsideRuntime(t *testing.T) {
	_, err := OpenArtifactStream(ArtifactOpenRequest{ObjectKey: "feeds/nvd/snapshot.zip"})
	if err == nil {
		t.Fatal("OpenArtifactStream() expected host error outside TinyGo runtime")
	}
	if hostErr, ok := err.(HostError); !ok || hostErr.Op != "artifact_open" {
		t.Fatalf("OpenArtifactStream() error = %#v, want HostError artifact_open", err)
	}
}

func TestArtifactStreamRejectsUninitializedUse(t *testing.T) {
	var stream ArtifactStream
	if _, err := stream.Write([]byte("chunk")); !errors.Is(err, errArtifactStreamNotInitialized) {
		t.Fatalf("Write() error = %#v, want errArtifactStreamNotInitialized", err)
	}
	if _, err := stream.Commit(ArtifactCommitRequest{}); !errors.Is(err, errArtifactStreamNotInitialized) {
		t.Fatalf("Commit() error = %#v, want errArtifactStreamNotInitialized", err)
	}
	if err := stream.Abort(); !errors.Is(err, errArtifactStreamNotInitialized) {
		t.Fatalf("Abort() error = %#v, want errArtifactStreamNotInitialized", err)
	}
}
