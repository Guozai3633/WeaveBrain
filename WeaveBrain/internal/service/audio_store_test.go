package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func newTestFileStore(t *testing.T) *LocalAudioFileStore {
	t.Helper()
	store, err := NewLocalAudioFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	return store
}

func TestLocalAudioFileStoreRoundTrip(t *testing.T) {
	store := newTestFileStore(t)
	assetID := uuid.New()
	storagePath := "audio/user1/asset.wav"

	if err := store.WriteChunk(context.Background(), assetID, 0, []byte("hello ")); err != nil {
		t.Fatalf("write chunk 0: %v", err)
	}
	if err := store.WriteChunk(context.Background(), assetID, 1, []byte("world")); err != nil {
		t.Fatalf("write chunk 1: %v", err)
	}

	finalPath, size, err := store.Finalize(context.Background(), assetID, storagePath, 2)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if size != 11 {
		t.Fatalf("expected size 11, got %d", size)
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		t.Fatalf("final file should exist: %v", err)
	}
	if info.Size() != 11 {
		t.Fatalf("final file size mismatch: %d", info.Size())
	}

	reader, err := store.OpenFinal(context.Background(), storagePath)
	if err != nil {
		t.Fatalf("open final: %v", err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if string(content) != "hello world" {
		t.Fatalf("roundtrip content mismatch: %q", string(content))
	}
}

func TestLocalAudioFileStoreWriteChunkIdempotent(t *testing.T) {
	store := newTestFileStore(t)
	assetID := uuid.New()

	if err := store.WriteChunk(context.Background(), assetID, 0, []byte("first")); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if err := store.WriteChunk(context.Background(), assetID, 0, []byte("second")); err != nil {
		t.Fatalf("rewrite chunk: %v", err)
	}

	_, size, err := store.Finalize(context.Background(), assetID, "audio/user1/a.wav", 1)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if size != 6 {
		t.Fatalf("expected retry to overwrite chunk, got size %d", size)
	}
}

func TestLocalAudioFileStoreFinalizeMissingChunk(t *testing.T) {
	store := newTestFileStore(t)
	assetID := uuid.New()
	if err := store.WriteChunk(context.Background(), assetID, 0, []byte("only chunk zero")); err != nil {
		t.Fatalf("write chunk: %v", err)
	}

	_, _, err := store.Finalize(context.Background(), assetID, "audio/user1/a.wav", 2)
	if err == nil {
		t.Fatalf("expected finalize to fail on missing chunk 1")
	}
}

func TestLocalAudioFileStoreOpenFinalRejectsTraversal(t *testing.T) {
	store := newTestFileStore(t)
	for _, path := range []string{
		"../escape.wav",
		"../../etc/passwd",
		"audio/../../outside.wav",
		"",
	} {
		if _, err := store.OpenFinal(context.Background(), path); err == nil {
			t.Fatalf("expected traversal path %q to be rejected", path)
		}
	}
}

func TestLocalAudioFileStoreFinalizeRejectsTraversal(t *testing.T) {
	store := newTestFileStore(t)
	_, _, err := store.Finalize(context.Background(), uuid.New(), "../../../escape.wav", 1)
	if err == nil {
		t.Fatalf("expected traversal storage path to be rejected")
	}
}

func TestLocalAudioFileStoreOpenFinalMissing(t *testing.T) {
	store := newTestFileStore(t)
	if _, err := store.OpenFinal(context.Background(), "audio/user1/nope.wav"); err == nil {
		t.Fatalf("expected error opening missing final file")
	}
}

func TestLocalAudioFileStoreRemoveStagingIdempotent(t *testing.T) {
	store := newTestFileStore(t)
	assetID := uuid.New()
	if err := store.WriteChunk(context.Background(), assetID, 0, []byte("x")); err != nil {
		t.Fatalf("write chunk: %v", err)
	}
	if err := store.RemoveStaging(context.Background(), assetID); err != nil {
		t.Fatalf("remove staging: %v", err)
	}
	// Second removal must be a no-op (missing dir).
	if err := store.RemoveStaging(context.Background(), assetID); err != nil {
		t.Fatalf("second remove staging: %v", err)
	}
	present, err := store.ListStagedChunks(assetID)
	if err != nil {
		t.Fatalf("list staged chunks: %v", err)
	}
	if len(present) != 0 {
		t.Fatalf("expected empty staged chunks after removal, got %v", present)
	}
}

func TestLocalAudioFileStoreListStagedChunks(t *testing.T) {
	store := newTestFileStore(t)
	assetID := uuid.New()

	empty, err := store.ListStagedChunks(assetID)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty, got %v", empty)
	}

	for _, idx := range []int32{0, 2, 5} {
		if err := store.WriteChunk(context.Background(), assetID, idx, []byte("x")); err != nil {
			t.Fatalf("write chunk %d: %v", idx, err)
		}
	}
	present, err := store.ListStagedChunks(assetID)
	if err != nil {
		t.Fatalf("list staged: %v", err)
	}
	for _, idx := range []int32{0, 2, 5} {
		if !present[idx] {
			t.Fatalf("expected chunk %d staged", idx)
		}
	}
}

func TestNewLocalAudioFileStoreRequiresBaseDir(t *testing.T) {
	if _, err := NewLocalAudioFileStore(""); err == nil {
		t.Fatalf("expected error for empty base dir")
	}
	// Parent dir creation must succeed.
	store, err := NewLocalAudioFileStore(filepath.Join(t.TempDir(), "nested", "audio"))
	if err != nil {
		t.Fatalf("nested dirs should be created: %v", err)
	}
	if store == nil {
		t.Fatalf("store must not be nil")
	}
}
