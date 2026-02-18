package cleaner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"go.uber.org/zap"
)

type Cleaner struct {
	downloadDir string
	dryRun      bool
	logger      *zap.Logger
}

type OrphanFile struct {
	Path  string
	Size  int64
	Links uint64
}

type CleanupResult struct {
	ScannedFiles int
	OrphanFiles  []OrphanFile
	FreedBytes   int64
	Errors       []error
}

func New(downloadDir string, dryRun bool, logger *zap.Logger) *Cleaner {
	return &Cleaner{
		downloadDir: downloadDir,
		dryRun:      dryRun,
		logger:      logger,
	}
}

// FindOrphans parcourt le downloadDir et retourne les fichiers
// avec un link count == 1 (plus aucun hardlink dans movies/ ou tv/)
func (c *Cleaner) FindOrphans() ([]OrphanFile, error) {
	var orphans []OrphanFile

	err := filepath.WalkDir(c.downloadDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// On ignore les dossiers
		if d.IsDir() {
			return nil
		}

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}

		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("cannot read syscall stat for %s", path)
		}

		// Fichier orphelin = plus aucun hardlink ailleurs
		if stat.Nlink == 1 {
			orphans = append(orphans, OrphanFile{
				Path:  path,
				Size:  info.Size(),
				Links: uint64(stat.Nlink),
			})
			c.logger.Info("orphan found",
				zap.String("path", path),
				zap.Int64("size_bytes", info.Size()),
			)
		}

		return nil
	})

	return orphans, err
}

// Cleanup supprime les fichiers orphelins et les dossiers vides
// Si DryRun == true, simule uniquement sans supprimer
func (c *Cleaner) Cleanup() (*CleanupResult, error) {
	result := &CleanupResult{}

	orphans, err := c.FindOrphans()
	if err != nil {
		return nil, fmt.Errorf("cleaner: find orphans: %w", err)
	}

	result.OrphanFiles = orphans

	for _, f := range orphans {
		result.ScannedFiles++

		if c.dryRun {
			c.logger.Info("dry-run: would delete",
				zap.String("path", f.Path),
				zap.Int64("size_bytes", f.Size),
			)
			result.FreedBytes += f.Size
			continue
		}

		if err := os.Remove(f.Path); err != nil {
			c.logger.Error("failed to delete orphan",
				zap.String("path", f.Path),
				zap.Error(err),
			)
			result.Errors = append(result.Errors, err)
			continue
		}

		c.logger.Info("deleted orphan",
			zap.String("path", f.Path),
			zap.Int64("size_bytes", f.Size),
		)
		result.FreedBytes += f.Size
	}

	// Nettoyage des dossiers vides après suppression des fichiers
	if !c.dryRun {
		if err := c.removeEmptyDirs(); err != nil {
			c.logger.Warn("failed to remove empty dirs", zap.Error(err))
		}
	}

	c.logger.Info("cleanup complete",
		zap.Int("scanned", result.ScannedFiles),
		zap.Int("orphans", len(result.OrphanFiles)),
		zap.Int64("freed_bytes", result.FreedBytes),
		zap.Int("errors", len(result.Errors)),
	)

	return result, nil
}

// FreedBytesHuman retourne la taille libérée en format lisible
func (r *CleanupResult) FreedBytesHuman() string {
	const unit = 1024
	b := r.FreedBytes
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ─── Private ──────────────────────────────────────────────────────────

func (c *Cleaner) removeEmptyDirs() error {
	return filepath.WalkDir(c.downloadDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == c.downloadDir {
			return err
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			c.logger.Info("removing empty dir", zap.String("path", path))
			return os.Remove(path)
		}

		return nil
	})
}
