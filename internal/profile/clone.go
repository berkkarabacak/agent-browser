package profile

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CopyStats counts regular files that are kept.
type CopyStats struct {
	Files   int
	Bytes   int64
	Skipped int
}

// CopyUserData copies a Chromium user-data directory, omitting caches,
// crashpad data, and singleton locks.
func CopyUserData(src, dst string) (CopyStats, error) {
	return walkUserData(src, func(rel, path string, d fs.DirEntry) error {
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyRegular(path, target, info.Mode())
	})
}

// MeasureUserData counts files that a clone would keep.
func MeasureUserData(src string) (CopyStats, error) {
	return walkUserData(src, func(rel, path string, d fs.DirEntry) error {
		return nil
	})
}

func walkUserData(src string, visit func(rel, path string, d fs.DirEntry) error) (CopyStats, error) {
	var stats CopyStats
	src = filepath.Clean(src)
	info, err := os.Stat(src)
	if err != nil {
		return stats, err
	}
	if !info.IsDir() {
		return stats, fmt.Errorf("%s is not a directory", src)
	}
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if ShouldSkip(rel, d.Type()) {
			stats.Skipped++
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			stats.Skipped++
			return nil
		}
		if !d.IsDir() && !d.Type().IsRegular() {
			stats.Skipped++
			return nil
		}
		if err := visit(rel, path, d); err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			stats.Files++
			stats.Bytes += info.Size()
		}
		return nil
	})
	return stats, err
}

func copyRegular(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	perm := mode.Perm()
	if perm == 0 {
		perm = 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// FormatBytes renders an approximate size for CLI output.
func FormatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	return formatUnsigned(uint64(n))
}

func formatUnsigned(n uint64) string {
	const kb = 1024
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < kb*kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	case n < kb*kb*kb:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(kb*kb))
	default:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(kb*kb*kb))
	}
}

// FormatRSS is FormatBytes for process working-set totals.
func FormatRSS(n uint64) string { return formatUnsigned(n) }

// SlashRel is exported for tests that build relative paths.
func SlashRel(parts ...string) string {
	return strings.Join(parts, "/")
}
