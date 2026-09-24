//go:build !tinygo

package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const localArtifactBody = "interface GigabitEthernet0/1\n ip address 192.0.2.1 255.255.255.0\n"

func localArtifactSHA(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func stageLocalArtifact(key, body, sha string) (*ArtifactCommitResponse, error) {
	stream, err := OpenArtifactStream(ArtifactOpenRequest{
		ObjectKey:   key,
		ContentType: "text/plain",
		SHA256:      sha,
		SizeBytes:   int64(len(body)),
		Attributes:  map[string]string{"kind": "running_config"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := stream.Write([]byte(body)); err != nil {
		_ = stream.Abort()
		return nil, err
	}
	return stream.Commit(ArtifactCommitRequest{SHA256: sha, SizeBytes: int64(len(body))})
}

func stagingLeftovers(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".artifact-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestLocalHostCommitsArtifactToDirectory(t *testing.T) {
	dir := t.TempDir()
	sha := localArtifactSHA(localArtifactBody)
	var committed *ArtifactCommitResponse

	capture, err := RunLocalHost(LocalHostOptions{ArtifactDir: dir}, func() error {
		response, err := stageLocalArtifact("plugin/running-config/1001", localArtifactBody, sha)
		committed = response
		return err
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}

	if committed.ObjectKey != "plugin/running-config/1001" || committed.SHA256 != sha ||
		committed.SizeBytes != int64(len(localArtifactBody)) || committed.ContentType != "text/plain" ||
		committed.Attributes["kind"] != "running_config" {
		t.Fatalf("commit response = %#v", committed)
	}

	path := filepath.Join(dir, "plugin", "running-config", "1001")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("committed artifact not on disk: %v", err)
	}
	if string(data) != localArtifactBody {
		t.Fatalf("artifact body = %q", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("artifact mode = %v, want 0600", info.Mode().Perm())
	}

	if len(capture.Artifacts) != 1 || capture.Artifacts[0].Path != path || capture.Artifacts[0].SHA256 != sha {
		t.Fatalf("capture artifacts = %#v", capture.Artifacts)
	}
	if leftovers := stagingLeftovers(t, dir); len(leftovers) != 0 {
		t.Fatalf("staging files left behind: %v", leftovers)
	}
}

func TestLocalHostWithoutArtifactDirReportsNoUploader(t *testing.T) {
	_, err := RunLocalHost(LocalHostOptions{}, func() error {
		_, err := stageLocalArtifact("plugin/running-config/1001", localArtifactBody, "")
		return err
	})
	var hostErr HostError
	if !errors.As(err, &hostErr) || hostErr.Code != hostErrNotFound {
		t.Fatalf("err = %v, want not-found host error", err)
	}
}

func TestLocalHostRejectsDigestMismatchWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	wrong := localArtifactSHA("some other body")

	_, err := RunLocalHost(LocalHostOptions{ArtifactDir: dir}, func() error {
		_, err := stageLocalArtifact("plugin/running-config/1001", localArtifactBody, wrong)
		return err
	})
	if err == nil {
		t.Fatal("expected digest mismatch to fail the commit")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "plugin", "running-config", "1001")); !os.IsNotExist(statErr) {
		t.Fatalf("artifact written despite digest mismatch: %v", statErr)
	}
	if leftovers := stagingLeftovers(t, dir); len(leftovers) != 0 {
		t.Fatalf("staging files left behind: %v", leftovers)
	}
}

func TestLocalHostRejectsUnsafeObjectKeys(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"", "/etc/passwd", "../escape", "plugin/../../escape", "plugin//double", "plugin/./dot", "plugin/sp ace"} {
		_, err := RunLocalHost(LocalHostOptions{ArtifactDir: dir}, func() error {
			_, err := OpenArtifactStream(ArtifactOpenRequest{ObjectKey: key})
			return err
		})
		if err == nil {
			t.Fatalf("object key %q was accepted", key)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected keys created files: %v", entries)
	}
}

func TestLocalHostDiscardsAbortedAndUncommittedStreams(t *testing.T) {
	dir := t.TempDir()

	_, err := RunLocalHost(LocalHostOptions{ArtifactDir: dir}, func() error {
		aborted, err := OpenArtifactStream(ArtifactOpenRequest{ObjectKey: "plugin/aborted"})
		if err != nil {
			return err
		}
		if _, err := aborted.Write([]byte(localArtifactBody)); err != nil {
			return err
		}
		if err := aborted.Abort(); err != nil {
			return err
		}

		abandoned, err := OpenArtifactStream(ArtifactOpenRequest{ObjectKey: "plugin/abandoned"})
		if err != nil {
			return err
		}
		_, err = abandoned.Write([]byte(localArtifactBody))
		return err
	})
	if err != nil {
		t.Fatalf("RunLocalHost() error = %v", err)
	}
	if leftovers := stagingLeftovers(t, dir); len(leftovers) != 0 {
		t.Fatalf("staging files left behind: %v", leftovers)
	}
	for _, name := range []string{"aborted", "abandoned"} {
		if _, statErr := os.Stat(filepath.Join(dir, "plugin", name)); !os.IsNotExist(statErr) {
			t.Fatalf("%s artifact was published: %v", name, statErr)
		}
	}
}
