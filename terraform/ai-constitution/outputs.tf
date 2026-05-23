output "project_name" {
  description = "Cloudflare Pages project name. Use as `wrangler pages deploy <dir> --project-name=<name>` in the deploy workflow."
  value       = cloudflare_pages_project.this.name
}

output "subdomain" {
  description = "Default Pages subdomain (e.g., ai-constitution.pages.dev). Useful for sanity-checking the project exists before the custom-domain CNAME resolves."
  value       = cloudflare_pages_project.this.subdomain
}

output "custom_domain" {
  description = "Custom hostname served by the Pages project."
  value       = var.custom_domain
}

output "site_url" {
  description = "Canonical site URL the public visits."
  value       = "https://${var.custom_domain}"
}
