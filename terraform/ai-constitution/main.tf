# aiConstitution methodology + spec site composition.
#
# Per SPEC.md §14.1, the canonical hostname is the kebab-case form
# (ai-constitution.convergent-systems.co); the camelCase form is a 301
# alias and is NOT managed here.
#
# Direct-upload Pages project: deployments arrive via
# `wrangler pages deploy ./web/ai-constitution/dist` in
# .github/workflows/deploy-ai-constitution.yml. Cloudflare does not pull
# from git directly.

module "pages" {
  source = "git::https://github.com/convergent-systems-co/core-infra.git//terraform/cloudflare/pages-project?ref=v0.1.0"

  cloudflare_account_id = var.cloudflare_account_id
  project_name          = var.project_name
  production_branch     = var.production_branch
  custom_domain         = var.custom_domain
  zone_id               = var.zone_id
}
