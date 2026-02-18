package cleaner

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func setupLogger(t *testing.T) *zap.Logger {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return logger
}

func TestFindOrphans_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, true, nil, setupLogger(t))

	orphans, err := c.FindOrphans()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(orphans))
	}
}

func TestCleanup_DryRun_DoesNotDelete(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "test.mkv")
	if err := os.WriteFile(testFile, []byte("fake video content"), 0644); err != nil {
		t.Fatal(err)
	}

	// nil pour qbit — pas besoin en dry-run.
	c := New(dir, true, nil, setupLogger(t))

	result, err := c.Cleanup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Le fichier doit encore exister en dry-run.
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("dry-run should not delete files")
	}

	if result.ScannedFiles != 1 {
		t.Errorf("expected 1 scanned file, got %d", result.ScannedFiles)
	}
}

func TestCleanup_DeletesOrphan(t *testing.T) {
	dir := t.TempDir()

	testFile := filepath.Join(dir, "orphan.mkv")
	if err := os.WriteFile(testFile, []byte("fake video content"), 0644); err != nil {
		t.Fatal(err)
	}

	// dry_run = false, qbit = nil.
	c := New(dir, false, nil, setupLogger(t))

	result, err := c.Cleanup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Le fichier doit avoir été supprimé.
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("orphan file should have been deleted")
	}

	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}

	if result.FreedBytes == 0 {
		t.Error("expected freed bytes > 0")
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
