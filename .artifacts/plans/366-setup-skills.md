# Plan: #366 — Setup TUI: Add Skill Selection Step

## Objective

After `ai setup` completes the wizard, show the user a numbered list of available skills and let them select which to install. Makes `ai setup` a complete one-shot install with no follow-up steps.

## Alternatives

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| New Bubble Tea TUI screen | Native to wizard UX | Complex; requires BubbleTea model changes; high blast radius | Rejected |
| Simple stdin readline prompt (chosen) | Minimal, testable via DI, no BubbleTea coupling | Less visually rich | Chosen |
| Always install all skills | Zero prompting | Unexpected side effects for users who don't want everything | Rejected |
| Skip entirely (do nothing) | Zero risk | Fails the acceptance criteria | Rejected |

## Scope

Files to create:
- `src/cmd/ai/cmd/setup_skills_prompt.go` — `runSkillSelectionPrompt` and supporting DI types
- `src/cmd/ai/cmd/setup_skills_prompt_test.go` — 4 tests

Files to modify:
- `src/cmd/ai/cmd/setup.go` — call `runSkillSelectionPrompt(os.Stdout, os.Stdin)` after wizard completes and before `runSetupPostWizard`

## Approach

1. Define injectable function types in `setup_skills_prompt.go`:
   - `fetchDirFn func() ([]skillAtomDirEntry, error)`
   - `fetchAtomFn func(url string) (*skillAtom, error)`
   - `installFn func(cmd *cobra.Command, slug string) error`

2. Implement `runSkillSelectionPrompt(w io.Writer, r io.Reader, fetch fetchDirFn, fetchAtom fetchAtomFn, install installFn, cmd *cobra.Command) error`:
   - Fetch all available skill atoms via injected functions
   - Print numbered list with box-drawing header
   - Read one line of input
   - Parse: "all" → all, "1,3,5" → subset, empty → skip, invalid token → warn and skip
   - Install selected skills; report progress; install errors are non-fatal (warn, continue)

3. Add `runSkillSelectionPromptReal(cmd *cobra.Command) error` as the real adapter that wires in `fetchSkillsDirectory`, `fetchSkillAtomFromURL`, `runSkillsInstall`.

4. In `runSetupTUI`: after `finalWizard.Done()` check passes and before `runSetupPostWizard`, add TTY guard and call `runSkillSelectionPromptReal(cmd)`. Errors are non-fatal (warn, continue to post-wizard).

5. `runSetupNonInteractive`: no change (skip entirely per spec).

## Testing Strategy

Four unit tests in `setup_skills_prompt_test.go`, all using mocks for fetch and install:

- `TestSkillSelectionPrompt_SelectAll` — input "all\n" → all slugs installed
- `TestSkillSelectionPrompt_SelectSubset` — input "1,3\n" → items 1 and 3 installed
- `TestSkillSelectionPrompt_Skip` — input "\n" → nothing installed
- `TestSkillSelectionPrompt_NonTTY` — `w` is not a TTY → skip entirely (this tests the guard logic by passing a non-TTY writer)

Since `term.IsTerminal` checks the fd of `os.Stdout` directly, and in tests we pass a `bytes.Buffer` (not a real fd), the TTY check in the function will need to accept an `io.Writer` and try-cast to `*os.File`; if not a `*os.File`, treat as non-TTY. This is the clean DI-friendly approach.

## Risk Assessment

- **Fetch fails**: non-fatal; prompt is skipped with a warning. Setup completes normally.
- **Install fails for one skill**: non-fatal; warning printed, setup continues.
- **Invalid input**: warn user, skip (no install). Setup continues.
- **Non-TTY**: guard skips prompt entirely. Tested.
- **Empty skill list from registry**: print "(no skills available)" and return without prompting.

## Backward Compatibility

- `runSetupNonInteractive` unchanged — no skill prompt, no breakage.
- `runSetupPostWizard` unchanged — no signature change.
- The skill selection is additive and non-fatal; existing tests are unaffected.

## Dependencies

None. Depends only on already-merged skills.go functions (merged in PR #369).
