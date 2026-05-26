package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// newOpCmd implements `ai op {env,signin,signout,whoami,clip}`.
//
// All subcommands delegate to the `op` CLI (1Password CLI). When `op` is
// not on PATH the commands return a descriptive error per the spec.
//
// Security notes (per Common.md §4):
//   - `ai op clip` MUST NOT write the secret value to stdout; it pipes
//     via a clipboard command (pbcopy / xclip / wl-copy).
//   - `ai op env` writes op:// references, never resolved secret values.
func newOpCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "op",
		Short: "1Password CLI integration (env, signin, signout, whoami, clip)",
		Long: `op provides thin, governance-aware wrappers around the 1Password CLI
(op). It never prints secret values to stdout; secrets travel only to
the clipboard (via clip) or remain as op:// references (via env).

Prerequisites: install the 1Password CLI from https://developer.1password.com/docs/cli/`,
	}

	c.AddCommand(
		newOpEnvCmd(),
		newOpSigninCmd(),
		newOpSignoutCmd(),
		newOpWhoamiCmd(),
		newOpClipCmd(),
	)
	return c
}

// requireOpBinary returns an error if the `op` binary is not on PATH.
func requireOpBinary() error {
	if _, err := exec.LookPath("op"); err != nil {
		return fmt.Errorf("op CLI not found — install 1Password CLI from https://developer.1password.com/docs/cli/")
	}
	return nil
}

// --- op env ---

// opItem is the minimal shape we need from `op item list --format json`.
type opItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Vault struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"vault"`
}

// envVarName converts an item title to a valid env-var name:
// upper-case, non-alphanumeric chars → underscore.
func envVarName(title string) string {
	re := regexp.MustCompile(`[^A-Z0-9]+`)
	return re.ReplaceAllString(strings.ToUpper(title), "_")
}

func newOpEnvCmd() *cobra.Command {
	var vault string
	var format string

	c := &cobra.Command{
		Use:   "env",
		Short: "Print op:// references as env-var assignments",
		Long: `env prints one KEY=op://vault/item/field line per item in 1Password.
The value is an op:// reference — a pointer, never the resolved secret.

Flags:
  --vault <name>           Filter by vault name.
  --format dotenv|export   Output format (default: dotenv).

Example:
  ai op env --vault Work | tee .env.op`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireOpBinary(); err != nil {
				return err
			}

			opArgs := []string{"item", "list", "--format", "json"}
			if vault != "" {
				opArgs = append(opArgs, "--vault", vault)
			}

			// gosec G204: args come from cobra flag parsing; not user-supplied exec.
			out, err := exec.Command("op", opArgs...).Output() //nolint:gosec
			if err != nil {
				return fmt.Errorf("op item list: %w", err)
			}

			var items []opItem
			if err := json.Unmarshal(out, &items); err != nil {
				return fmt.Errorf("op item list: parse JSON: %w", err)
			}

			w := cmd.OutOrStdout()
			for _, item := range items {
				// Client-side vault filter (op CLI may not support --vault on all
				// versions; belt-and-suspenders).
				if vault != "" && !strings.EqualFold(item.Vault.Name, vault) {
					continue
				}
				name := envVarName(item.Title)
				ref := fmt.Sprintf("op://%s/%s/", item.Vault.Name, item.ID)
				var line string
				if format == "export" {
					line = fmt.Sprintf("export %s=%s\n", name, ref)
				} else {
					line = fmt.Sprintf("%s=%s\n", name, ref)
				}
				fmt.Fprint(w, line)
			}
			return nil
		},
	}

	c.Flags().StringVar(&vault, "vault", "", "filter by vault name")
	c.Flags().StringVar(&format, "format", "dotenv", "output format: dotenv or export")
	return c
}

// --- op signin ---

func newOpSigninCmd() *cobra.Command {
	var address string

	c := &cobra.Command{
		Use:   "signin",
		Short: "Sign in to 1Password (prints eval instruction or runs op account add)",
		Long: `signin cannot exec directly in the parent shell; it prints the eval
instruction instead.

With --address <addr>:
  Runs ` + "`" + `op account add --address <addr>` + "`" + ` to add a new account.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if address == "" {
				// Cannot eval in the parent shell from a subprocess.
				// No op binary needed — just printing the instruction.
				fmt.Fprintln(cmd.OutOrStdout(), "Run the following in your shell to sign in:") //nolint:errcheck
				fmt.Fprintln(cmd.OutOrStdout(), "  eval $(op signin)")                        //nolint:errcheck
				return nil
			}

			if err := requireOpBinary(); err != nil {
				return err
			}

			// gosec G204: address comes from cobra flag parsing.
			c := exec.Command("op", "account", "add", "--address", address) //nolint:gosec
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			return c.Run()
		},
	}

	c.Flags().StringVar(&address, "address", "", "1Password account address (e.g. my.1password.com)")
	return c
}

// --- op signout ---

func newOpSignoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "signout",
		Short: "Sign out of 1Password (runs op signout --forget)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireOpBinary(); err != nil {
				return err
			}
			c := exec.Command("op", "signout", "--forget") //nolint:gosec
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			return c.Run()
		},
	}
}

// --- op whoami ---

func newOpWhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Print current 1Password account info (runs op whoami --format json)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireOpBinary(); err != nil {
				return err
			}
			c := exec.Command("op", "whoami", "--format", "json") //nolint:gosec
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			return c.Run()
		},
	}
}

// --- op clip ---

// clipboardCmd returns the clipboard writer command for the current OS and
// environment. Returns ("", error) when no clipboard writer is found.
func clipboardCmd() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		if p, err := exec.LookPath("pbcopy"); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("clipboard command not found: pbcopy not on PATH")
	default:
		// Linux: try wl-copy (Wayland) then xclip (X11).
		if p, err := exec.LookPath("wl-copy"); err == nil {
			return p, nil
		}
		if p, err := exec.LookPath("xclip"); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("clipboard command not found: neither wl-copy nor xclip on PATH")
	}
}

func newOpClipCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clip <ref>",
		Short: "Copy a 1Password secret to the clipboard (never prints to stdout)",
		Long: `clip reads an op:// reference (or item name) via ` + "`" + `op read` + "`" + ` and pipes
the result directly to the OS clipboard command. The secret value is NEVER
written to stdout (per Common.md §4, non-overridable).

Example:
  ai op clip op://Private/MyDB/password`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireOpBinary(); err != nil {
				return err
			}

			clipPath, err := clipboardCmd()
			if err != nil {
				return err
			}

			ref := args[0]

			// Read the secret from op; capture to pipe directly to clipboard.
			// gosec G204: ref is validated by cobra (ExactArgs); it comes from
			// the user's own shell invocation of `ai op clip`.
			opCmd := exec.Command("op", "read", ref) //nolint:gosec
			opOut, err := opCmd.Output()
			if err != nil {
				return fmt.Errorf("op read: %w", err)
			}

			// Pipe to clipboard — do not print secret to stdout.
			// gosec G204: clipPath resolved via exec.LookPath above.
			clipCmd := exec.Command(clipPath) //nolint:gosec
			if runtime.GOOS != "darwin" {
				// xclip requires -selection clipboard flag.
				if strings.HasSuffix(clipPath, "xclip") {
					clipCmd = exec.Command(clipPath, "-selection", "clipboard") //nolint:gosec
				}
			}
			clipCmd.Stdin = strings.NewReader(string(opOut))
			if err := clipCmd.Run(); err != nil {
				return fmt.Errorf("clipboard write failed: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Secret copied to clipboard.")
			return nil
		},
	}
}
