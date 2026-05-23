output "project_name" {
  description = "Cloudflare Pages project name. Use as the second positional arg to `wrangler pages deploy <dir> --project-name=<name>` in the deploy workflow."
  value       = module.pages.project_name
}

output "subdomain" {
  description = "Default Pages subdomain (e.g., ai-constitution.pages.dev). Useful for sanity-checking the project exists before the custom-domain CNAME resolves."
  value       = module.pages.subdomain
}

output "custom_domain" {
  description = "Custom hostname served by the Pages project."
  value       = module.pages.custom_domain
}

output "site_url" {
  description = "Canonical site URL the public visits."
  value       = "https://${var.custom_domain}"
}
