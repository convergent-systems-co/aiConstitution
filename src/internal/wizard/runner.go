package wizard

import "fmt"

// RunNonInteractive executes the wizard using seeded answers (no user input).
// It iterates active questions in dependency order, using seeds when available,
// falling back to the question's Default. Returns an error if a required
// question has neither a seed nor a default.
func RunNonInteractive(tax Taxonomy, seeds map[string]string) (map[string]string, error) {
	if seeds == nil {
		seeds = make(map[string]string)
	}
	answers := make(map[string]string)

	// Iterative resolution: keep walking until no new answers are added.
	// This handles chained dependencies (A unlocks B unlocks C).
	for {
		added := false
		for _, q := range tax.ActiveQuestions(answers) {
			if _, done := answers[q.ID]; done {
				continue
			}
			val, ok := seeds[q.ID]
			if !ok && q.Default != "" {
				val = q.Default
				ok = true
			}
			if !ok {
				if q.Required {
					return nil, fmt.Errorf("wizard: required question %q has no seeded answer or default", q.ID)
				}
				continue
			}
			answers[q.ID] = val
			added = true
		}
		if !added {
			break
		}
	}

	return answers, nil
}
