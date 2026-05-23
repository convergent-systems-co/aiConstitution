# aiConstitution methodology + spec site composition.
#
# Per SPEC.md §14.1, the canonical hostname is the kebab-case form
# (ai-constitution.convergent-systems.co); the camelCase form is a
# 301 alias and is NOT managed here.
#
# Direct-upload Pages project: deployments arrive via
# `wrangler pages deploy ./web/ai-constitution/dist` in
# .github/workflows/deploy-ai-constitution.yml. Cloudflare does not
# pull from git directly.
#
# Three resources, two providers (see providers.tf for the split-token
# rationale):
#
#   provider = cloudflare.account → Pages project + Pages domain
#   provider = cloudflare.dns     → proxied CNAME on convergent-systems.co
#
# This inlines what
# convergent-systems-co/core-infra//terraform/cloudflare/pages-project@v0.1.0
# does, with the post-2026-05-23 split-token format applied. Once
# core-infra ships a v0.2.0 of pages-project with provider aliases
# baked in, this composition migrates back to the module call.

locals {
  has_custom_domain = var.custom_domain != "" && var.zone_id != ""
}

# Direct-upload Pages project.
resource "cloudflare_pages_project" "this" {
  provider = cloudflare.account

  account_id        = var.cloudflare_account_id
  name              = var.project_name
  production_branch = var.production_branch
}

# Custom-hostname attachment.
resource "cloudflare_pages_domain" "custom" {
  count    = local.has_custom_domain ? 1 : 0
  provider = cloudflare.account

  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.this.name
  name         = var.custom_domain
}

# Proxied CNAME pointing the custom domain at the Pages default
# subdomain. Orange-cloud on: TLS termination, caching,
# custom-domain certificate auto-provisioning.
resource "cloudflare_dns_record" "pages_cname" {
  count    = local.has_custom_domain ? 1 : 0
  provider = cloudflare.dns

  zone_id = var.zone_id
  name    = var.custom_domain
  type    = "CNAME"
  content = "${var.project_name}.pages.dev"
  proxied = true
  ttl     = 1 # 1 == "auto" — required when proxied

  comment = "Cloudflare Pages — managed by aiConstitution//terraform/ai-constitution"

  depends_on = [cloudflare_pages_domain.custom]
}
