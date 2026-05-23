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

For the Cloudflare provider itself you need `CLOUDFLARE_API_TOKEN`
with these scopes:

- **Account → Cloudflare Pages → Edit**
- **Zone → DNS → Edit** on the `convergent-systems.co` zone

See `core-infra/terraform/cloudflare/pages-token/` and
`core-infra/terraform/cloudflare/dns-token/` for the token-minting
modules.

## Workflow

1. **First-time setup** — copy `terraform.tfvars.example` →
   `terraform.tfvars`; fill in `zone_id`.
2. **Plan** — `cs-tofu -chdir=terraform/ai-constitution init &&
   cs-tofu -chdir=terraform/ai-constitution plan`.
3. **Apply** — `cs-tofu -chdir=terraform/ai-constitution apply`.
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
