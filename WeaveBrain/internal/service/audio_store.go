package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// AudioFileStore abstracts chunk staging and final audio file persistence.
//
// All paths exchanged with the store are LOGICAL paths relative to the store
// root (e.g. "audio/{userID}/{assetID}.wav"). They are stored verbatim in the
// audio_assets.storage_path column and are used by OpenFinal. The store maps
// logical paths to its physical base directory internally.
type AudioFileStore interface {
	// WriteChunk persists one chunk part, overwriting on retry so re-sending a
	// chunk is idempotent.
	WriteChunk(ctx context.Context, assetID uuid.UUID, chunkIndex int32, data []byte) error
	// Finalize concatenates staged parts into the final audio file located at
	// storagePath and returns the physical path and its size.
	Finalize(ctx context.Context, assetID uuid.UUID, storagePath string, totalChunks int32) (finalPath string, size int64, err error)
	// OpenFinal opens the final audio file at the given logical storage path.
	OpenFinal(ctx context.Context, storagePath string) (io.ReadCloser, error)
	// RemoveStaging deletes all staged chunk parts.
	RemoveStaging(ctx context.Context, assetID uuid.UUID) error
	// ListStagedChunks returns the set of chunk indices currently staged.
	ListStagedChunks(assetID uuid.UUID) (map[int32]bool, error)
}

// LocalAudioFileStore stores chunk parts under baseDir/staging/{assetID}/ and
// final files under baseDir/final/. Safe for the MVP local deployment.
type LocalAudioFileStore struct {
	baseDir string
}

func NewLocalAudioFileStore(baseDir string) (*LocalAudioFileStore, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("audio store base dir is required")
	}
	for _, sub := range []string{"staging", "final"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("create audio store dir: %w", err)
		}
	}
	return &LocalAudioFileStore{baseDir: baseDir}, nil
}

func (s *LocalAudioFileStore) stagingDir(assetID uuid.UUID) string {
	return filepath.Join(s.baseDir, "staging", assetID.String())
}

func (s *LocalAudioFileStore) chunkPath(assetID uuid.UUID, chunkIndex int32) string {
	return filepath.Join(s.stagingDir(assetID), fmt.Sprintf("part-%d", chunkIndex))
}

func (s *LocalAudioFileStore) WriteChunk(
	_ context.Context,
	assetID uuid.UUID,
	chunkIndex int32,
	data []byte,
) error {
	dir := s.stagingDir(assetID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create staging dir: %w", err)
	}
	path := s.chunkPath(assetID, chunkIndex)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write audio chunk: %w", err)
	}
	return nil
}

func (s *LocalAudioFileStore) Finalize(
	_ context.Context,
	assetID uuid.UUID,
	storagePath string,
	totalChunks int32,
) (string, int64, error) {
	if storagePath == "" {
		return "", 0, fmt.Errorf("invalid final audio path: storage path is required")
	}
	finalPath, err := s.physicalPath(storagePath)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return "", 0, fmt.Errorf("create final audio dir: %w", err)
	}
	out, err := os.Create(finalPath)
	if err != nil {
		return "", 0, fmt.Errorf("create final audio file: %w", err)
	}
	defer out.Close()

	var size int64
	for i := int32(0); i < totalChunks; i++ {
		partPath := s.chunkPath(assetID, i)
		part, err := os.Open(partPath)
		if err != nil {
			out.Close()
			os.Remove(finalPath)
			return "", 0, fmt.Errorf("finalize missing chunk %d: %w", i, err)
		}
		n, copyErr := io.Copy(out, part)
		part.Close()
		if copyErr != nil {
			out.Close()
			os.Remove(finalPath)
			return "", 0, fmt.Errorf("finalize chunk %d: %w", i, copyErr)
		}
		size += n
	}
	if err := out.Sync(); err != nil {
		return "", 0, fmt.Errorf("finalize sync: %w", err)
	}
	return finalPath, size, nil
}

func (s *LocalAudioFileStore) OpenFinal(
	_ context.Context,
	storagePath string,
) (io.ReadCloser, error) {
	finalPath, err := s.physicalPath(storagePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(finalPath)
	if err != nil {
		return nil, fmt.Errorf("open final audio: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("open final audio: path is a directory")
	}
	f, err := os.Open(finalPath)
	if err != nil {
		return nil, fmt.Errorf("open final audio: %w", err)
	}
	return f, nil
}

// physicalPath resolves a logical storage path under the store base directory,
// rejecting traversal attempts that escape the base directory.
func (s *LocalAudioFileStore) physicalPath(storagePath string) (string, error) {
	if storagePath == "" {
		return "", fmt.Errorf("invalid final audio path: storage path is required")
	}
	clean := filepath.Clean(filepath.Join(s.baseDir, storagePath))
	if clean != s.baseDir && !strings.HasPrefix(clean, s.baseDir+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid final audio path: escapes base directory")
	}
	return clean, nil
}

func (s *LocalAudioFileStore) RemoveStaging(
	_ context.Context,
	assetID uuid.UUID,
) error {
	dir := s.stagingDir(assetID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read staging dir: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "part-") {
			if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
				return fmt.Errorf("remove staged chunk: %w", err)
			}
		}
	}
	return nil
}

// ListStagedChunks returns the set of chunk indices currently staged (used by
// Complete to detect missing chunks before finalizing).
func (s *LocalAudioFileStore) ListStagedChunks(assetID uuid.UUID) (map[int32]bool, error) {
	dir := s.stagingDir(assetID)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[int32]bool{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read staging dir: %w", err)
	}
	present := make(map[int32]bool, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "part-") {
			continue
		}
		index := strings.TrimPrefix(name, "part-")
		var i int
		if _, err := fmt.Sscanf(index, "%d", &i); err == nil {
			present[int32(i)] = true
		}
	}
	return present, nil
}

// stagedChunkIndices returns sorted staged chunk indices.
func stagedChunkIndices(present map[int32]bool) []int32 {
	indices := make([]int32, 0, len(present))
	for i := range present {
		indices = append(indices, i)
	}
	sort.Slice(indices, func(a, b int) bool { return indices[a] < indices[b] })
	return indices
}
