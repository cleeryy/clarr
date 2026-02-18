//go:build !windows

package cleaner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"go.uber.org/zap"
)

// FindOrphans parcourt le downloadDir et retourne les fichiers
// avec un link count == 1 (plus aucun hardlink dans movies/ ou tv/).
func (c *Cleaner) FindOrphans() ([]OrphanFile, error) {
	var orphans []OrphanFile

	err := filepath.WalkDir(c.downloadDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

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
