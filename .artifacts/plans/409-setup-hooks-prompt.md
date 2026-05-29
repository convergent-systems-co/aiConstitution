# Plan: Hook Selection Step in TUI Wizard (#409)

**Objective:** Add a hook selection TUI step to `ai setup` so users can choose which governance hooks to install during setup — mirroring the existing skill selection step.

## Alternatives Table

| Option | Tradeoff |
|--------|----------|
| Add hook selection TUI step (chosen) | Consistent UX with skills, lets users choose hooks during setup |
| Auto-install all hooks silently | Simpler but gives no user agency; breaks the "interactive" intent |
| Separate `ai hooks setup` command | More explicit but adds friction; setup should be one-stop |

## Scope

- **Create:** `src/cmd/ai/cmd/setup_hooks_prompt.go`
- **Create:** `src/cmd/ai/cmd/setup_hooks_prompt_test.go`
- **Modify:** `src/cmd/ai/cmd/setup.go` (wire before skill selection in `runSetupTUI`)

## Approach

1. Create `setup_hooks_prompt.go` mirroring `setup_skills_prompt.go`:
   - `hookDescriptions` map: hardcoded one-liners for each embedded hook
   - `runHookSelectionPrompt(w, r, isTTY, hooksDir, installHook, wireHooks, home)` — injectable for testing
   - `runHookSelectionPromptReal()` — binds real embed/install functions

2. Wire into `setup.go` `runSetupTUI` BEFORE skill selection, gated on `!noHooks`

3. Leave `runSetupPostWizard` unchanged — `ExtractAllHooks` with `overwrite=false` is idempotent and installs any hooks the user didn't select

4. Write tests first (TDD):
   - `TestHookSelectionPrompt_InstallsSelected`
   - `TestHookSelectionPrompt_All`
   - `TestHookSelectionPrompt_Skip`
   - `TestHookSelectionPrompt_NonTTY`
   - `TestHookSelectionPrompt_InvalidSelection`

## Testing Strategy

Mock `hookInstallFn` and `hookWireFn` in tests. Use `embed.HookNames()` to dynamically build the hook list, so tests stay green as hooks are added.

## Risk Assessment

- **Risk:** `embed.HookNames()` returns different set in tests vs production. Mitigation: tests use the real embed package and count returned hooks.
- **Risk:** `installClaudeHooks` takes `(repoRoot, hooksDir)` not `(claudeDir, hooksDir)`. The wire function matches that signature.

## Backward Compatibility

- `runSetupPostWizard` still calls `ExtractAllHooks(overwrite=false)` — installs any hooks the user didn't select
- `--no-hooks` flag gates both the new prompt and the existing post-wizard install
- Non-TTY path is unaffected
