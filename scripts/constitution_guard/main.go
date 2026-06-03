package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	ai, err := exec.LookPath("ai")
	if err != nil {
		fmt.Fprintln(os.Stderr, "constitution-guard: ai executable not found on PATH; build/install ai or run through the project CLI")
		os.Exit(1)
	}
	cmd := exec.Command(ai, append([]string{"guard"}, os.Args[1:]...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "constitution-guard: run ai guard: %v\n", err)
		os.Exit(1)
	}
}
