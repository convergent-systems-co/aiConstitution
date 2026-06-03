# AI Constitution Panel Review

Date: 2026-06-03

Worktree: `/Users/itsfwcp/workspace/convergent-system-co/aiConstitution/.worktrees/codex-copilot-parity`

## Method

Three read-only review passes were run with different model overrides and expert lenses:

- AI Constitution governance expert
- Skills Designer expert
- Hooks/Git enforcement expert

The findings below are aggregated, deduplicated, and written as issue-ready recommendations.

## Filing Status

Issue creation was attempted through both `gh issue create` and the GitHub connector.

- `gh issue create`: `GraphQL: Unauthorized: As an Enterprise Managed User, you cannot access this content (createIssue)`
- GitHub connector: `FORBIDDEN: Resource not accessible by integration`

The project issue bodies were prepared under `/private/tmp/aiconstitution-issue-01.md` through `/private/tmp/aiconstitution-issue-12.md`.

## Recommended Issues

### 1. Unify cross-agent instruction templates across Claude, Codex, Copilot, and Cursor

Severity: High

The project has multiple independently-authored instruction templates for Claude, Codex, Copilot, and Cursor. This creates parity drift: some paths load the compact constitution plus operational rules, while others still point at legacy/runtime artifacts.

Evidence:

- `AGENTS.md` and `.github/copilot-instructions.md` duplicate operating rules.
- `src/cmd/ai/cmd/init.go` still writes `.claude/CLAUDE.md` with `@~/.ai/Constitution.md` and points Cursor/Copilot at `Constitution.runtime.md`.
- `src/cmd/ai/cmd/integrate.go` uses `Constitution.compact.md` plus Codex-specific rules.
- `src/cmd/ai/internal/init/init.go` has another Copilot/AGENTS template body.
- Tests in `src/cmd/ai/cmd/init_test.go` preserve the older strings.

Acceptance criteria:

- `ai init`, `ai setup`, `ai init-integrate`, and internal bootstrap files derive from one shared template/model.
- Claude, Copilot, Cursor, and Codex load the same intended constitution form unless an exception is documented.
- Generated-output tests assert parity across all integration entrypoints.
- A tool integration matrix documents intentional differences.

### 2. Wire Copilot/Cursor/Codex health checks into `ai doctor` and `ai status`

Severity: High

`ai doctor` and `ai status` do not surface the existing Copilot, Cursor, and Codex integration checks, so broken or stale integrations can remain invisible.

Evidence:

- `src/cmd/ai/cmd/doctor.go` defines `checkDoctorCopilot`, `checkDoctorCursor`, and `checkDoctorAgentsMD`.
- The main `runDoctor` flow does not invoke those checks.
- `src/cmd/ai/cmd/status.go` inline warnings only inspect constitution files and hook files.
- Tests currently cover helper functions rather than the public command output.

Acceptance criteria:

- `ai doctor` reports Copilot symlink state, Cursor rule state, and Codex `AGENTS.md` include state when those surfaces are present.
- `ai status` either reports those same warnings or clearly delegates to `ai doctor`.
- Tests cover present, missing, and stale integration states through the public command surfaces.
- Remediation messages name the exact command to repair each tool.

### 3. Resolve Copilot path and artifact mismatch

Severity: High

Copilot integration paths and source artifacts are not aligned. Different code paths reference repo-level `.github/copilot-instructions.md`, `~/.ai/.github/copilot-instructions.md`, and `~/.copilot/instructions/constitution.md`, with both compact and runtime constitution artifacts mentioned.

Acceptance criteria:

- A single Copilot integration contract documents each supported target path and when it is used.
- Compact vs runtime constitution selection is deliberate, documented, and tested.
- `ai init-integrate`, `ai hooks install --copilot`, `ai setup`, and `ai doctor` agree on expected state.
- Stale legacy Copilot artifacts are detected or migrated.

### 4. Centralize protected branch and worktree policy

Severity: High

Protected branch/worktree policy is duplicated in code even though `governance/policy/branch-guard.json` claims to be canonical.

Acceptance criteria:

- All guards and worktree creators read the same protected-branch policy.
- Custom protected names and patterns are covered by tests.
- Worktree base branch selection is configurable or derived from repository default branch.
- No independent hard-coded protected branch lists remain outside default policy definitions.

### 5. Replace environment-variable mutation approval with auditable approval records

Severity: High

Mutation approval is represented by bare environment variables such as `AI_CONSTITUTION_APPROVED_MUTATION=1`. That is easy for a tool path to self-assert and does not leave an auditable approval record.

Acceptance criteria:

- Approvals include actor, timestamp, scope, command/ref/worktree target, and source entrypoint.
- The guard verifies approval scope rather than only checking an environment variable.
- Allow and deny decisions write audit events.
- Tests prove an approval for one command or ref cannot authorize a different mutation.

### 6. Make repo-managed Git hooks distribution-safe and cross-platform

Severity: High

Repo-managed Git hooks and generated hook installation paths are not yet a distribution-safe cross-platform enforcement plane.

Acceptance criteria:

- Git hooks execute without requiring Go to be installed.
- POSIX and Windows hook templates use supported invocation paths.
- `ai clone` and `ai hooks install --repo` install consistent hook sets.
- `commit-msg` trailer enforcement is installed or otherwise covered wherever pre-commit enforcement is installed.
- Tests cover generated hook behavior on POSIX and Windows template paths.

### 7. Enforce `~/.ai/bin` PATH presence and order in `ai doctor`

Severity: High

Wrapper enforcement depends on `~/.ai/bin` being early in `PATH`, but `ai doctor` does not currently report missing or shadowed wrapper placement even though the check exists.

Acceptance criteria:

- `ai doctor` warns when `~/.ai/bin` is absent from `PATH`.
- `ai doctor` warns when `~/.ai/bin` is present but shadowed by system Git/GitHub CLI binaries.
- Remediation includes `ai hooks install command-wrappers` and shell-specific PATH guidance.
- Tests cover missing and shadowed PATH cases.

### 8. Implement documented `command-wrappers.local.toml` merge behavior

Severity: Medium

Wrapper configuration comments promise `~/.config/aiConstitution/command-wrappers.local.toml` merging, but the loader only reads the canonical config and fallback embedded file.

Acceptance criteria:

- Runtime wrapper config loads local extensions from the documented config path.
- Merge precedence and append/override behavior are documented.
- Tests prove local pre/post hooks append without mutating canonical entries.
- Invalid local config fails with a clear diagnostic.

### 9. Harden wrapper real-binary resolution

Severity: Medium

Wrapper real-binary resolution accepts any non-directory PATH candidate and only skips exact `paths.BinDir()` string matches. This is fragile around symlinked bins, non-executable files, and equivalent path spellings.

Acceptance criteria:

- Resolve and compare PATH entries against canonical `paths.BinDir()` using cleaned/real paths.
- Reject self-path aliases and symlinks back into the wrapper directory.
- Require executable regular-file candidates.
- Tests cover symlinked `~/.ai/bin`, shadowed PATH entries, and non-executable placeholders.

### 10. Normalize audit hook naming

Severity: Medium

Hook health checks disagree on whether `audit.py` or `audit-logger.py` is canonical, which can create false warnings during install, doctor, or status checks.

Acceptance criteria:

- Install, doctor, status, docs, and tests agree on the canonical audit hook name.
- Existing `audit.py` installs or wirings are recognized as legacy-compatible.
- A regression test proves status is green after the current default hook install.

### 11. Normalize skills discovery and install through one catalog pipeline

Severity: Medium

Skill discovery and install surfaces do not share one normalized catalog pipeline. Users can see different availability, filtering, dependency, and install behavior depending on which command they run.

Acceptance criteria:

- `available`, setup picker, `install`, and `install --all` consume the same normalized catalog model.
- Deprecated/retired skills and sub-skills are filtered consistently.
- Registry migration/fallback behavior is documented once.
- Tests cover namespaced dependencies and consistent filtering across commands.

### 12. Implement skill bundle installation or narrow the contract

Severity: Medium

The product contract describes skills as bundles with templates/assets, but registry installation currently materializes only `SKILL.md`. Template commands therefore cannot work for registry-installed skills that include supporting files.

Acceptance criteria:

- Registry install fetches and materializes templates/assets alongside `SKILL.md`, or CLI/docs explicitly state that registry installs are prompt-fragment only.
- `ai skills templates list/show` works for at least one registry-installed skill with templates.
- Tests cover a registry-installed skill with a real `templates/` payload.
