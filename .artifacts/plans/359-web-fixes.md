# Plan: Issue #359 — Website fixes

**Objective:** Fix four issues in the aiConstitution website and deploy workflow.

## Scope

Files to create/modify:
- `SPEC.md` (create at repo root)
- `web/ai-constitution/src/pages/install.astro` (modify)
- `web/ai-constitution/src/pages/setup.astro` (modify)
- `.github/workflows/deploy-ai-constitution.yml` (modify)

## Approach

1. Create `SPEC.md` at repo root — fixes `/spec` route 404 (content.config.ts globs for `SPEC.md`)
2. Fix `install.astro`: remove winget section; change "8 questions" wizard description
3. Fix `setup.astro`: remove "~14 questions" claim
4. Fix deploy workflow: add Cloudflare credential validation step; pin action to commit SHA

## Testing strategy

Build site: `cd web/ai-constitution && npm run build`
- Verify `/spec/index.html` exists in dist
- Verify no `winget` in install page output
- Verify no "8 questions" in install page output
- Verify no "14 questions" in setup page output

## Alternatives

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Update content.config.ts to glob the actual spec file | Correct source | Breaks naming convention; spec file name embeds version | Rejected |
| Create SPEC.md at root | Simple; follows existing convention for other docs | Requires manual update when spec changes | Chosen |

## Risk assessment

- SPEC.md is static — will need updating when spec version bumps. Mitigated by the note pointing to the actual spec file.
- Pinning action SHA: if action-atoms cuts a new release, workflow must be updated. This is intentional supply-chain hygiene.
