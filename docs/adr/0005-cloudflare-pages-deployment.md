# 0005. Deploy ai-constitution.convergent-systems.co on Cloudflare Pages via core-infra modules

- Status: accepted
- Date: 2026-05-23
- Source: SPEC.md §14, conversation 2026-05-23 (deploy + DNS for ai-constitution)
- Supersedes: none
- Related: ADR-0001 (atoms architecture — explains why each *-atoms site
  ships in its own repo with its own terraform; this repo only owns the
  ai-constitution composition)

## Context

`web/ai-constitution/` is an Astro static site that needs to be
publicly served at `ai-constitution.convergent-systems.co` per
SPEC.md §14.1. The Convergent Systems estate already standardizes on:

- Cloudflare Pages for static-site hosting (per `core-infra`
  conventions and the `pages-project` module).
- Cloudflare R2 for state storage (`cs-tfstate` bucket, provisioned
  by `core-infra/terraform/cloudflare/state-bucket/`).
- OpenTofu (1.12.0 at the time of writing) as the IaC engine.
- The `convergent-systems-co/core-infra` repo as the source of shared
  modules, consumed via `git::` source refs with tag-pinned `ref=`.

We needed to decide which of those conventions to use for this repo's
infrastructure, and how to compose them.

## Decision

1. **Infrastructure plane lives at `terraform/ai-constitution/`** as a
   single OpenTofu composition. No envs/ / modules/ split here — this
   repo has one site, one environment.
2. **Modules are consumed from `core-infra` via git source refs**:
   `source = "git::https://github.com/convergent-systems-co/core-infra.git//terraform/cloudflare/pages-project?ref=v0.1.0"`.
   Pinned to tag `v0.1.0`; bumps are deliberate.
3. **State backend is the shared `cs-tfstate` R2 bucket** owned by
   `core-infra`. Backend key follows the repo-scoped pattern
   `state-bucket/convergent-systems-co/aiConstitution/<composition>.tfstate`.
4. **State locking** uses OpenTofu's `use_lockfile = true` (S3
   conditional writes against a lock object). R2 supports this; no
   DynamoDB-style lock table is involved.
5. **Site content delivery** is direct-upload via
   `npx wrangler pages deploy` in
   `.github/workflows/deploy-ai-constitution.yml`. Cloudflare does
   NOT git-clone the repo — the workflow builds the Astro artifact,
   wrangler uploads.
6. **DNS** is a proxied CNAME from
   `ai-constitution.convergent-systems.co` to
   `ai-constitution.pages.dev`, managed by the `pages-project`
   module's `cloudflare_dns_record.pages_cname` resource.

## Consequences

- **Reproducible** — anyone with the `cs-tofu` wrapper and the
  appropriate Cloudflare token can `cs-tofu -chdir=terraform/ai-constitution apply`
  and reproduce the same Pages project + DNS topology.
- **Shared upgrade path** — when `core-infra` ships a new
  `pages-project` version (e.g., adds bot protection, edge rules),
  bumping is a one-line `?ref=` change.
- **No state-bucket bootstrap in this repo** — the `cs-tfstate`
  bucket already exists; we just need a token scoped to read/write
  the `state-bucket/convergent-systems-co/aiConstitution/` prefix.
- **Deployments are separable from infrastructure** — wrangler pushes
  content; Terraform owns the project + DNS. Either can change
  without the other.
- **The four atom-registry sites stay out** of this composition.
  `brand-atoms.com`, `persona-atoms.com`, `profile-atoms.com`,
  `skill-atoms.com` each ship in their own repo with their own
  composition (per ADR-0001). This terraform only owns
  `ai-constitution.convergent-systems.co`.

## Alternatives considered

| Alternative | Pros | Cons | Verdict |
|---|---|---|---|
| Cloudflare Pages **with Git source binding** | One fewer workflow; CF auto-builds on push | The build environment is opaque; secrets are harder to inject; PR previews route through CF instead of GitHub-controlled CI. core-infra deliberately picked direct-upload to keep build control with Actions. | Rejected — matching core-infra's convention. |
| Vendoring `pages-project` into this repo | No remote git fetch at `init` time; truly offline-buildable | Loses the shared-module benefit; bug fixes upstream have to be cherry-picked manually; drift over time. | Rejected — explicit "core-infra method and modules" directive from the principal. |
| Separate `envs/{prod,staging}/` tree | Future-proofs for a staging environment | YAGNI: one site, one env. Adding a staging dir later is a one-time copy. | Rejected — keep flat until needed. |
| Skip Terraform; just `wrangler pages project create` once | Less code | Project + custom domain + DNS not in IaC; future drift invisible. | Rejected — non-IaC infra is unaccountable. |
| Use a DynamoDB lock table (AWS) for state locking | Battle-tested | Requires standing up AWS for a single lock — defeats the all-Cloudflare premise. | Rejected — `use_lockfile = true` on R2 is sufficient (OpenTofu 1.10+). |

## Token scopes required

For `cs-tofu -chdir=terraform/ai-constitution apply` to succeed:

- **Cloudflare API token** with:
  - Account → Cloudflare Pages → Edit
  - Zone → DNS → Edit (on the `convergent-systems.co` zone)
- **R2 credentials** (AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY env
  vars) scoped to read/write `cs-tfstate`.

Both are minted by core-infra modules (`pages-token`, `dns-token`,
`storage-token`). The `cs-tofu` wrapper loads them from 1Password.

For `.github/workflows/deploy-ai-constitution.yml` to succeed:

- `CLOUDFLARE_API_TOKEN` GitHub Actions secret (Pages Edit scope).
- `CLOUDFLARE_ACCOUNT_ID` GitHub Actions secret.

These are SEPARATE from the terraform tokens — wrangler doesn't need
DNS or R2 scopes.
