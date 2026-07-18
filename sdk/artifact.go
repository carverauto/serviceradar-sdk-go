package sdk

import (
	"context"
	"encoding/json"
	"errors"
)

const (
	// CapabilityArtifactStagingV1 marks plugins that stage durable artifacts
	// through the host runtime. The host brokers storage through agent-gateway.
	CapabilityArtifactStagingV1 = "artifact-staging:v1"
)

var errArtifactStreamNotInitialized = errors.New("artifact stream not initialized")

// ArtifactOpenRequest describes an artifact stream the host should broker to
// the ServiceRadar object path through agent-gateway.
type ArtifactOpenRequest struct {
	ObjectKey   string         `json:"object_key,omitempty"`
	ArtifactID  string         `json:"artifact_id,omitempty"`
	Type        string         `json:"type,omitempty"`
	ContentType string         `json:"content_type,omitempty"`
	Format      string         `json:"format,omitempty"`
	SHA256      string         `json:"sha256,omitempty"`
	SizeBytes   int64          `json:"size_bytes,omitempty"`
	SourceURL   string         `json:"source_url,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ArtifactChunkMetadata describes one chunk in a host-brokered artifact stream.
type ArtifactChunkMetadata struct {
	Sequence uint64         `json:"sequence,omitempty"`
	Offset   int64          `json:"offset,omitempty"`
	SHA256   string         `json:"sha256,omitempty"`
	IsFinal  bool           `json:"is_final,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ArtifactCommitRequest asks the host to finalize the stream and verify
// producer-supplied integrity metadata before accepting the object.
type ArtifactCommitRequest struct {
	SHA256    string         `json:"sha256,omitempty"`
	SizeBytes int64          `json:"size_bytes,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ArtifactCommitResponse is returned by the host after a successful commit.
type ArtifactCommitResponse struct {
	ObjectKey      string         `json:"object_key"`
	ArtifactID     string         `json:"artifact_id,omitempty"`
	SHA256         string         `json:"sha256,omitempty"`
	ContentType    string         `json:"content_type,omitempty"`
	Format         string         `json:"format,omitempty"`
	SizeBytes      int64          `json:"size_bytes,omitempty"`
	StorageBackend string         `json:"storage_backend,omitempty"`
	Accepted       bool           `json:"accepted"`
	Status         string         `json:"status,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ArtifactStream wraps a host-managed durable artifact stream.
type ArtifactStream struct {
	handle uint32
}

// OpenArtifactStream opens a durable artifact stream through the host.
func OpenArtifactStream(req ArtifactOpenRequest) (*ArtifactStream, error) {
	return OpenArtifactStreamContext(context.Background(), req)
}

// OpenArtifactStreamContext opens a durable artifact stream through the host.
func OpenArtifactStreamContext(ctx context.Context, req ArtifactOpenRequest) (*ArtifactStream, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	res := hostArtifactOpen(ptrFromBytes(payload), uint32(len(payload)))
	if res < 0 {
		return nil, hostErr(res, "artifact_open")
	}

	return &ArtifactStream{handle: uint32(res)}, nil
}

// Write uploads an artifact chunk to the host-managed stream.
func (s *ArtifactStream) Write(meta ArtifactChunkMetadata, payload []byte) error {
	return s.WriteContext(context.Background(), meta, payload)
}

// WriteContext uploads an artifact chunk to the host-managed stream.
func (s *ArtifactStream) WriteContext(ctx context.Context, meta ArtifactChunkMetadata, payload []byte) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	if s == nil || s.handle == 0 {
		return errArtifactStreamNotInitialized
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	res := hostArtifactWrite(
		s.handle,
		ptrFromBytes(metaJSON),
		uint32(len(metaJSON)),
		ptrFromBytes(payload),
		uint32(len(payload)),
	)

	return hostErr(res, "artifact_write")
}

// Commit finalizes the stream and returns the host-accepted object metadata.
func (s *ArtifactStream) Commit(req ArtifactCommitRequest) (*ArtifactCommitResponse, error) {
	return s.CommitContext(context.Background(), req)
}

// CommitContext finalizes the stream and returns the host-accepted object metadata.
func (s *ArtifactStream) CommitContext(ctx context.Context, req ArtifactCommitRequest) (*ArtifactCommitResponse, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}

	if s == nil || s.handle == 0 {
		return nil, errArtifactStreamNotInitialized
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	for _, size := range []int{4096, 16384, 65536} {
		respBuf := make([]byte, size)
		res := hostArtifactCommit(
			s.handle,
			ptrFromBytes(reqJSON),
			uint32(len(reqJSON)),
			ptrFromBytes(respBuf),
			uint32(len(respBuf)),
		)
		if res == hostErrTooLarge {
			continue
		}
		if err := hostErr(res, "artifact_commit"); err != nil {
			return nil, err
		}
		if res > int32(len(respBuf)) {
			return nil, HostError{Code: hostErrTooLarge, Op: "artifact_commit"}
		}

		s.handle = 0

		var response ArtifactCommitResponse
		if err := json.Unmarshal(respBuf[:res], &response); err != nil {
			return nil, err
		}

		return &response, nil
	}

	return nil, HostError{Code: hostErrTooLarge, Op: "artifact_commit"}
}

// Abort cancels the host-managed artifact stream.
func (s *ArtifactStream) Abort(reason string) error {
	if s == nil || s.handle == 0 {
		return nil
	}

	reasonBytes := []byte(reason)
	res := hostArtifactAbort(s.handle, ptrFromBytes(reasonBytes), uint32(len(reasonBytes)))
	s.handle = 0

	return hostErr(res, "artifact_abort")
}
