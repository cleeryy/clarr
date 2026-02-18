package cleaner

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func TestFindOrphans_EmptyDir(t *testing.T) {
	dir := setupTestDir(t)
	logger, _ := zap.NewDevelopment()
	c := New(dir, true, logger)

	orphans, err := c.FindOrphans()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestCleanup_DryRun(t *testing.T) {
	dir := setupTestDir(t)

	// Créer un fichier de test (links == 1 par défaut)
	testFile := filepath.Join(dir, "test.mkv")
	if err := os.WriteFile(testFile, []byte("fake video content"), 0644); err != nil {
		t.Fatal(err)
	}

	logger, _ := zap.NewDevelopment()
	c := New(dir, true, logger) // dry_run = true

	result, err := c.Cleanup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Le fichier doit encore exister en dry-run
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("dry-run should not delete files")
	}

	if result.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", result.ScannedFiles)
	}
}

func TestFreedBytesHuman(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		r := &CleanupResult{FreedBytes: tt.bytes}
		if got := r.FreedBytesHuman(); got != tt.expected {
			t.Errorf("FreedBytesHuman(%d) = %q, want %q", tt.bytes, got, tt.expected)
		}
	}
}
