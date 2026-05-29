# aiConstitution — AI Tool Project Context

This repo is the **tool** half of the two-repo AI governance system.

## Two-repo model

| Repo | What it is | Who modifies it |
|---|---|---|
| `convergent-systems-co/aiConstitution` (this repo) | Go CLI binary, hooks, skill templates, CI | Engineers via PRs |
| `convergent-systems-co/ai` (personal) | User's Constitution.md, memory, audit logs | AI assistant + user |

The binary reads from `~/.ai/` (synced from `convergent-systems-co/ai`). It does NOT commit to `~/.ai/`.

## What's in this repo

```
src/cmd/ai/          Go CLI binary (cobra commands)
src/internal/        Internal packages
plugins/             Plugin artifact directories (manifest.yaml + SKILL.md)
governance/          Policy files, seed data, wizard config
questions.yaml       Setup wizard question tree
settings.toml.example Example user settings
governance/schemas/  JSON Schemas validating config files
.github/workflows/   CI (lint, test, build, release)
```

## Build

```bash
make build           # produces dist/ai
./dist/ai version
```

## Test

```bash
make test
# or
go test ./src/...
```

## Key contracts

- **Never commit user data** — Constitution.md, memory, audit logs belong in `~/.ai/`, not here.
- **Never commit secrets** — API keys, tokens, `.env` files. Use `~/.ai/hooks/` for secret redaction.
- **Conventional Commits** — feat/fix/refactor/chore/docs/test/ci prefixes required.
- **CI must pass** before merge — lint (golangci-lint), test (go test), build.

## Governance

This repo follows `~/.ai/Constitution.md` (the unified single-file governance document). The AI assistant working in this repo loads it via `~/.claude/CLAUDE.md`.
