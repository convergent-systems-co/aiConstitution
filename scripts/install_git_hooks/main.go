package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "install-git-hooks: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	hooksPath, err := gitOutput("rev-parse", "--git-path", "hooks")
	if err != nil {
		return fmt.Errorf("resolve hooks path: %w", err)
	}
	if err := os.MkdirAll(hooksPath, 0o750); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	hooks := []string{
		"pre-commit",
		"pre-push",
		"commit-msg",
		"pre-commit.ps1",
		"pre-push.ps1",
		"commit-msg.ps1",
	}
	for _, hook := range hooks {
		src := filepath.Join("scripts", "git-hooks", hook)
		dst := filepath.Join(hooksPath, hook)
		if err := copyFile(src, dst, modeForHook(hook)); err != nil {
			return err
		}
		fmt.Printf("installed %s\n", dst)
	}
	return nil
}

func modeForHook(name string) os.FileMode {
	if filepath.Ext(name) == ".ps1" {
		return 0o644
	}
	return 0o755
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(filepath.Clean(src))
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(filepath.Clean(dst), data, mode); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return string(bytesTrimSpace(out)), nil
}

func bytesTrimSpace(data []byte) []byte {
	for len(data) > 0 && (data[0] == '\n' || data[0] == '\r' || data[0] == '\t' || data[0] == ' ') {
		data = data[1:]
	}
	for len(data) > 0 {
		last := data[len(data)-1]
		if last != '\n' && last != '\r' && last != '\t' && last != ' ' {
			break
		}
		data = data[:len(data)-1]
	}
	return data
}
