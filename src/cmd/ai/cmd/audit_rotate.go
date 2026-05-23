package cmd

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newAuditRotateCmd implements `ai audit rotate`. Replaces the
// previous bin/audit-rotate.sh script.
//
// Gzips every <YYYY-MM>.jsonl under ~/.ai/audit/interactions/ except
// the current UTC month. Idempotent.
func newAuditRotateCmd() *cobra.Command {
	var dryRun bool

	c := &cobra.Command{
		Use:   "rotate",
		Short: "Gzip audit/interactions/<YYYY-MM>.jsonl files older than the current month",
		Long: `rotate compresses the prior months' interaction-audit JSONL
files. Per SPEC.md §17.4 (risk: audit logs accumulate noise).

  ~/.ai/audit/interactions/<YYYY-MM>.jsonl    → kept (current month)
  ~/.ai/audit/interactions/<YYYY-MM-OLD>.jsonl → gzipped to *.jsonl.gz

Idempotent. Suitable for invocation from cron / launchd / Task
Scheduler.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			root := os.Getenv("AI_ROOT")
			if root == "" {
				root = filepath.Join(home, ".ai")
			}
			dir := filepath.Clean(filepath.Join(root, "audit", "interactions"))
			info, err := os.Stat(dir)
			if err != nil {
				if os.IsNotExist(err) {
					notice("audit rotate:", dir, "does not exist; nothing to do.")
					return nil
				}
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("audit rotate: %s is not a directory", dir)
			}

			currentMonth := time.Now().UTC().Format("2006-01")
			entries, err := os.ReadDir(dir)
			if err != nil {
				return err
			}
			rotated := 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if !strings.HasSuffix(name, ".jsonl") {
					continue
				}
				base := strings.TrimSuffix(name, ".jsonl")
				if base == currentMonth {
					continue
				}
				src := filepath.Clean(filepath.Join(dir, name))
				dst := src + ".gz"
				if dryRun {
					fmt.Printf("[would gzip] %s → %s\n", src, dst)
					rotated++
					continue
				}
				if err := gzipFile(src, dst); err != nil {
					return fmt.Errorf("audit rotate: %s: %w", src, err)
				}
				if err := os.Remove(src); err != nil {
					return fmt.Errorf("audit rotate: remove %s: %w", src, err)
				}
				fmt.Printf("[gzipped] %s\n", dst)
				rotated++
			}
			if rotated == 0 {
				notice("audit rotate: nothing to do.")
			}
			return nil
		},
	}

	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be gzipped without writing")
	return c
}

// gzipFile reads src and writes a gzipped copy to dst (level 9).
func gzipFile(src, dst string) error {
	in, err := os.Open(filepath.Clean(src))
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	// 0o600: only the user needs to read their own audit log;
	// avoids gosec G302 (file-perms-too-permissive).
	out, err := os.OpenFile(filepath.Clean(dst), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	if _, err := io.Copy(gz, in); err != nil {
		return err
	}
	return nil
}
