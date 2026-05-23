variable "cloudflare_account_id" {
  description = "Cloudflare account ID hosting the Pages project. The Convergent Systems org account; same value as in every core-infra composition (see core-infra/terraform/cloudflare/*/versions.tf backend endpoints — the subdomain before .r2.cloudflarestorage.com IS the account id)."
  type        = string
}

variable "zone_id" {
  description = "Cloudflare zone ID for the convergent-systems.co zone. The Pages project's custom-domain CNAME lands here. Look up via `cf-api zones?name=convergent-systems.co` or read from core-infra/terraform/cloudflare/dns-token outputs."
  type        = string
}

variable "project_name" {
  description = "Cloudflare Pages project name. Also the default URL: https://<project_name>.pages.dev. Hyphen-separated; alphanumeric + hyphen only."
  type        = string
  default     = "ai-constitution"
}

variable "custom_domain" {
  description = "Custom hostname the Pages project serves traffic for. Per SPEC.md §14.1 the canonical hostname is the kebab-case form."
  type        = string
  default     = "ai-constitution.convergent-systems.co"
}

variable "production_branch" {
  description = "Branch that triggers production deploys via wrangler pages deploy in .github/workflows/deploy-ai-constitution.yml. Direct-upload model — Cloudflare itself does not pull from git."
  type        = string
  default     = "main"
}

# Split tokens per the 2026-05-23 narrow-token convention (see
# providers.tf). Both are CONCEALED in 1Password; the cs-tofu wrapper
# loads them as TF_VAR_account_token / TF_VAR_dns_token before exec.
# Marked sensitive so plan/apply output doesn't echo them.

variable "account_token" {
  description = "Cloudflare API token with Account → Cloudflare Pages → Edit. From 1Password 'Convergent Systems - Account'."
  type        = string
  sensitive   = true
}

variable "dns_token" {
  description = "Cloudflare API token with Zone → DNS → Edit on convergent-systems.co. From 1Password 'Convergent Systems - DNS'."
  type        = string
  sensitive   = true
}
