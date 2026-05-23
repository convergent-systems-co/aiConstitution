# Split-token provider configurations per the 2026-05-23 narrow-token
# convention from convergent-systems-co/core-infra.
#
# Two cloudflare provider instances via aliases:
#
#   cloudflare.account  — uses the "Convergent Systems - Account" token,
#                         scoped to Account → Cloudflare Pages → Edit.
#                         Drives cloudflare_pages_project +
#                         cloudflare_pages_domain.
#
#   cloudflare.dns      — uses the "Convergent Systems - DNS" token,
#                         scoped to Zone → DNS → Edit on
#                         convergent-systems.co. Drives
#                         cloudflare_dns_record for the proxied CNAME.
#
# The earlier draft of this composition went through the
# core-infra//terraform/cloudflare/pages-project@v0.1.0 module, which
# pre-dates the token split: a single unaliased provider tries to do
# both account-level and zone-level work and authentication fails for
# whichever scope the loaded token doesn't have. The split-token
# format is the post-2026-05-23 convention; we follow it directly
# until pages-project ships a v0.2.0 that exposes provider aliases.
#
# Tokens are provided via:
#   TF_VAR_account_token   — set by cs-tofu wrapper (or shell export)
#   TF_VAR_dns_token       — set by cs-tofu wrapper (or shell export)

provider "cloudflare" {
  alias     = "account"
  api_token = var.account_token
}

provider "cloudflare" {
  alias     = "dns"
  api_token = var.dns_token
}
