#!/usr/bin/env bash
# sync-labels.sh — Idempotent label sync for convergent-systems-co repos
#
# Creates the full kind/* label set in aiConstitution and all *atom* repos.
# Safe to re-run: uses --force which upserts (create or update).
#
# Usage:
#   ./scripts/sync-labels.sh
#   GH_TOKEN=$(gh auth token --user polliard) ./scripts/sync-labels.sh
#
# Requirements: gh CLI, authenticated with write access to convergent-systems-co

set -euo pipefail

ORG="convergent-systems-co"
PRIMARY_REPO="${ORG}/aiConstitution"

# ---------------------------------------------------------------------------
# Label definitions — parallel arrays of (name, color, description)
# ---------------------------------------------------------------------------
# color #d4c5f9 matches the existing kind/finding scheme
KIND_COLOR="d4c5f9"

# Each entry is "name|description" — avoids associative array key issues with /
LABELS=(
  "kind/epic|Top-level organizational container for a body of work"
  "kind/feature|User-facing capability addition"
  "kind/story|User-level description of a desired behavior"
  "kind/task|Atomic unit of implementation work"
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

log() { printf '[sync-labels] %s\n' "$*" >&2; }

ensure_labels() {
  local repo="$1"
  log "Syncing labels → ${repo}"
  for entry in "${LABELS[@]}"; do
    local name="${entry%%|*}"
    local desc="${entry#*|}"
    gh label create "${name}" \
      --repo "${repo}" \
      --color "${KIND_COLOR}" \
      --description "${desc}" \
      --force >/dev/null 2>&1 || {
      log "ERROR: failed to upsert label '${name}' in ${repo}"
      return 1
    }
    log "  upserted: ${name}"
  done
}

# ---------------------------------------------------------------------------
# Discover atom repos
# ---------------------------------------------------------------------------

log "Discovering atom repos in ${ORG}…"
ATOM_REPOS=()
while IFS= read -r name; do
  ATOM_REPOS+=("${ORG}/${name}")
done < <(
  gh repo list "${ORG}" --json name --jq '.[].name' \
    | grep "atom" \
    | sort
)
log "Found ${#ATOM_REPOS[@]} atom repos"

# ---------------------------------------------------------------------------
# Apply labels
# ---------------------------------------------------------------------------

# Primary repo first
ensure_labels "${PRIMARY_REPO}"

# All atom repos
for repo in "${ATOM_REPOS[@]}"; do
  ensure_labels "${repo}"
done

log "Done. All labels synced."
