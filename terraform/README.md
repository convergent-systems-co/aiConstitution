# terraform/

OpenTofu infrastructure for the `aiConstitution` repo.

Pinned to **OpenTofu 1.12.0** (matches `convergent-systems-co/core-infra`'s
`.opentofu-version`; lockfile-based state locking requires ≥ 1.10).

## Layout

```
terraform/
└── ai-constitution/        composition for ai-constitution.convergent-systems.co
    ├── versions.tf         provider pins + R2 backend
    ├── variables.tf        account_id, zone_id, project_name, custom_domain
    ├── main.tf             calls core-infra//terraform/cloudflare/pages-project@v0.1.0
    ├── outputs.tf          project_name, subdomain, custom_domain, site_url
    └── terraform.tfvars.example
```

## Conventions

The compositions in this repo are thin consumers of shared modules
hosted in `convergent-systems-co/core-infra` (private). The pattern:

```hcl
module "pages" {
  source = "git::https://github.com/convergent-systems-co/core-infra.git//terraform/cloudflare/pages-project?ref=v0.1.0"
  …
}
```

Module versions are pinned to a tag (`v0.1.0` today). Bumps are
deliberate and reviewed.

Every composition's state lives in the `cs-tfstate` R2 bucket
(provisioned by core-infra, NOT here). Backend keys follow the pattern:

```
state-bucket/convergent-systems-co/aiConstitution/<composition>.tfstate
```

State locking uses OpenTofu's `use_lockfile = true` (S3 conditional
writes against a lock object) — no DynamoDB-style external lock table
needed; R2 has no native one.

## Credentials

The R2 backend reads `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY`
from the environment. These are R2-scoped tokens, not real AWS
credentials. The convergent canonical way to load them is the
`cs-tofu` wrapper from `core-infra/scripts/`, which pulls a 1Password
vault item into the env before exec'ing `tofu`:

```bash
cs-tofu -chdir=terraform/ai-constitution plan
cs-tofu -chdir=terraform/ai-constitution apply
```

**Split-token format (2026-05-23 onward).** Each composition uses
**two** Cloudflare provider configurations via aliases (see
`ai-constitution/providers.tf`):

- `cloudflare.account` ← `var.account_token` ← `TF_VAR_account_token`
  from `op item get "Convergent Systems - Account"`.
  Scope: **Account → Cloudflare Pages → Edit**.
- `cloudflare.dns`     ← `var.dns_token`     ← `TF_VAR_dns_token`
  from `op item get "Convergent Systems - DNS"`.
  Scope: **Zone → DNS → Edit** on `convergent-systems.co`.

The `cs-tofu` wrapper at `~/bin/cs-tofu` does the 1Password fetch
automatically for known module names. The `ai-constitution`
composition isn't in the wrapper's mapping yet, so you have two
choices:

1. **Export manually** before `cs-tofu`:
   ```bash
   export TF_VAR_account_token=$(op item get "Convergent Systems - Account" --vault Developer --fields credential --reveal)
   export TF_VAR_dns_token=$(op item get "Convergent Systems - DNS" --vault Developer --fields credential --reveal)
   cs-tofu -chdir=terraform/ai-constitution plan
   ```
2. **Patch cs-tofu** to map `ai-constitution` → both tokens
   (mirroring the `auth)` case in core-infra's wrapper). One-line
   change; left as a follow-up.

See `core-infra/terraform/cloudflare/pages-token/` and
`core-infra/terraform/cloudflare/dns-token/` for the token-minting
modules.

## Workflow

1. **First-time setup** — copy `terraform.tfvars.example` →
   `terraform.tfvars`. `zone_id` and `cloudflare_account_id` are
   already filled with the canonical values; nothing to edit unless
   you're pointing at a non-canonical account.
2. **Load tokens** (see "Split-token format" above):
   ```bash
   export TF_VAR_account_token=$(op item get "Convergent Systems - Account" --vault Developer --fields credential --reveal)
   export TF_VAR_dns_token=$(op item get "Convergent Systems - DNS" --vault Developer --fields credential --reveal)
   ```
3. **Plan** — `cs-tofu -chdir=terraform/ai-constitution init &&
   cs-tofu -chdir=terraform/ai-constitution plan`.
4. **Apply** — `cs-tofu -chdir=terraform/ai-constitution apply`.
   Creates the Pages project + custom-domain attachment + proxied
   CNAME on `convergent-systems.co`.
4. **Deploy site content** — pushed to `main` triggers
   `.github/workflows/deploy-ai-constitution.yml`, which runs
   `npm run build && wrangler pages deploy dist --project-name=ai-constitution`.

The Pages project is created by Terraform; deployments are by
Wrangler. The separation matches the convention in core-infra
(direct-upload Pages, no git-source binding).

## What is NOT in this terraform

- The R2 state bucket (`cs-tfstate`) — owned by `core-infra`.
- The Cloudflare API tokens — minted by `core-infra/terraform/cloudflare/{pages-token,dns-token}/`.
- The DNS zone `convergent-systems.co` — managed by `core-infra`.
- The four atom-registry sites (`brand-atoms.com`, `persona-atoms.com`,
  `profile-atoms.com`, `skill-atoms.com`) — each lives in its own repo
  and has its own composition. This terraform only covers
  `ai-constitution.convergent-systems.co`.

Per SPEC.md §14: each Convergent Systems Astro property carries its
own deploy concerns.
