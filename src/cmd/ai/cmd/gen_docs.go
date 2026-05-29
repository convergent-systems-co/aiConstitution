//go:build ignore

package main

import (
	"fmt"
	"sort"

	cmd "github.com/convergent-systems-co/aiConstitution/src/cmd/ai/cmd"
)

func main() {
	root := cmd.NewRootCmd()
	commands := root.Commands()
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name() < commands[j].Name()
	})
	fmt.Println("| Command | Purpose |")
	fmt.Println("|---|---|")
	for _, c := range commands {
		if c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		fmt.Printf("| `ai %s` | %s |\n", c.Name(), c.Short)
	}
}
