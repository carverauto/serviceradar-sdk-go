package sdk

import (
	"errors"
	"testing"
)

func TestCameraMediaStreamRequiresHandle(t *testing.T) {
	t.Parallel()

	var stream *CameraMediaStream
	if err := stream.Write(CameraMediaChunkMetadata{Sequence: 1}, []byte("frame")); !errors.Is(err, errCameraMediaStreamNotInitialized) {
		t.Fatalf("expected camera media handle error, got %v", err)
	}

	if err := stream.Heartbeat(CameraMediaHeartbeat{Sequence: 1}); !errors.Is(err, errCameraMediaStreamNotInitialized) {
		t.Fatalf("expected camera media heartbeat handle error, got %v", err)
	}
}
