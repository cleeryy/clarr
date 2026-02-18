//go:build windows

package cleaner

// FindOrphans is not supported on Windows.
// Hardlink detection requires Unix syscalls.
func (c *Cleaner) FindOrphans() ([]OrphanFile, error) {
	c.logger.Warn("orphan detection is not supported on Windows")
	return nil, nil
}
