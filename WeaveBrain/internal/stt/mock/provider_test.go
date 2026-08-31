package mock

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRecognizeFileReturnsScriptedResult(t *testing.T) {
	p := NewProvider(Config{Script: []string{"今天天气很好"}, Delay: 0})
	path := writeTempAudio(t, []byte("some fake wav bytes"))

	result, err := p.RecognizeFile(context.Background(), path)
	if err != nil {
		t.Fatalf("recognize: %v", err)
	}
	if result.Text != "今天天气很好" {
		t.Fatalf("expected scripted text, got %q", result.Text)
	}
	if !result.IsFinal {
		t.Fatalf("file recognition must be final")
	}
	if result.Confidence == 0 {
		t.Fatalf("expected non-zero confidence")
	}
}

func TestRecognizeFileMissingFile(t *testing.T) {
	p := NewProvider(Config{})
	_, err := p.RecognizeFile(context.Background(), filepath.Join(t.TempDir(), "nope.wav"))
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestRecognizeFileEmptyFile(t *testing.T) {
	p := NewProvider(Config{})
	path := writeTempAudio(t, nil)
	_, err := p.RecognizeFile(context.Background(), path)
	if err == nil {
		t.Fatalf("expected error for empty file")
	}
}

func TestRecognizeFileSimulatedFailure(t *testing.T) {
	p := NewProvider(Config{})
	path := writeTempAudio(t, []byte("contains FAIL marker"))
	_, err := p.RecognizeFile(context.Background(), path)
	if err == nil {
		t.Fatalf("expected simulated failure when file contains FAIL")
	}
}

func TestRecognizeFileNoScript(t *testing.T) {
	p := &Provider{script: nil}
	path := writeTempAudio(t, []byte("bytes"))
	_, err := p.RecognizeFile(context.Background(), path)
	if err == nil {
		t.Fatalf("expected error when no script configured")
	}
}

func writeTempAudio(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write test audio: %v", err)
	}
	return path
}
