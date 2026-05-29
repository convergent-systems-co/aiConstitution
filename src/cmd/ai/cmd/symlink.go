package cmd

import (
	"io"
	"os"
	"runtime"
)

// symlinkOrCopy creates a symlink from link→target. On Windows where symlinks
// require Developer Mode, it falls back to a file copy so the tool always works.
func symlinkOrCopy(target, link string) error {
	// Remove existing link/file before creating.
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		// On POSIX, symlink should always work — return the real error.
		return os.Symlink(target, link)
	}
	// Windows fallback: copy the file content.
	src, err := os.Open(target)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(link)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}
