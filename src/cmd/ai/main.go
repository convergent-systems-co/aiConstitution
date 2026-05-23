// Package main is the entry point for the `ai` CLI binary.
//
// `ai` operationalizes the four-file AI Constitution governance system
// (Constitution / Common / Code / Writing) — see SPEC.md at the repo
// root for the authoritative implementation specification.
//
// All command implementations live in the cmd/ subdirectory.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
