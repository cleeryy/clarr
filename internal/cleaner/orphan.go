package cleaner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cleeryy/clarr/internal/qbittorrent"
	"go.uber.org/zap"
)

type Cleaner struct {
	downloadDir string
	dryRun      bool
	qbit        *qbittorrent.Client
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

func New(downloadDir string, dryRun bool, qbit *qbittorrent.Client, logger *zap.Logger) *Cleaner {
	return &Cleaner{
		downloadDir: downloadDir,
		dryRun:      dryRun,
		qbit:        qbit,
		logger:      logger,
	}
}

// Cleanup supprime les fichiers orphelins, notifie qBittorrent
// et nettoie les dossiers vides.
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

		if c.qbit != nil {
			if err := c.qbit.DeleteTorrentByPath(f.Path, false); err != nil {
				c.logger.Warn("qbittorrent torrent not removed",
					zap.String("path", f.Path),
					zap.Error(err),
				)
			} else {
				c.logger.Info("qbittorrent torrent removed",
					zap.String("path", f.Path),
				)
			}
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

// FreedBytesHuman retourne la taille libérée en format lisible.
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
