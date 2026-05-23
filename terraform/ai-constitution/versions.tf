terraform {
  required_version = ">= 1.10.0"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
  }

  # State lives in the Convergent Systems R2 state bucket (`cs-tfstate`).
  # The bucket itself is provisioned by `convergent-systems-co/core-infra`
  # via `terraform/cloudflare/state-bucket/` — we are a consumer, not the
  # owner. If the bucket is rotated or moved, this block must be updated
  # in lockstep with every other composition's backend.
  #
  # Credentials: AWS_ACCESS_KEY_ID + AWS_SECRET_ACCESS_KEY env vars holding
  # an R2 token scoped to read/write `cs-tfstate`. See
  # core-infra/scripts/bootstrap-tf-state.sh + cs-tofu wrapper for the
  # 1Password-backed credential flow.
  #
  # use_lockfile = true → OpenTofu 1.10+ uses S3 conditional writes against
  # a lock object alongside the state object, giving us concurrent-apply
  # safety without a separate DynamoDB-style lock table (R2 has no such
  # service of its own).
  backend "s3" {
    bucket = "cs-tfstate"
    key    = "state-bucket/convergent-systems-co/aiConstitution/ai-constitution.tfstate"
    region = "auto"
    endpoints = {
      s3 = "https://e1fe0f0ce8ff18da4edc118372c30022.r2.cloudflarestorage.com"
    }
    skip_credentials_validation = true
    skip_region_validation      = true
    skip_metadata_api_check     = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
    use_path_style              = false
    use_lockfile                = true
  }
}
