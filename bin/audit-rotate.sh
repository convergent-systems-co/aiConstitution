#!/usr/bin/env bash
# bin/audit-rotate.sh — monthly rotation of audit/interactions/*.jsonl.
#
# JSONL audit logs grow without bound; this script gzips files older
# than the current month and prunes files older than ~/.config/
# aiConstitution/settings.toml [audit] retentionMonths (TBD; for now
# the rotation just gzips).
#
# Usage:
#   audit-rotate.sh            # rotate audit/interactions/
#   audit-rotate.sh --dry-run  # show what would happen
#
# Per SPEC.md §17.4 risk table ("audit logs accumulate noise").

set -euo pipefail

AI_ROOT="${AI_ROOT:-$HOME/.ai}"
DRY_RUN="no"

[[ "${1:-}" == "--dry-run" ]] && DRY_RUN="yes"

interactions="$AI_ROOT/audit/interactions"
if [[ ! -d "$interactions" ]]; then
  exit 0
fi

current_month=$(date -u +"%Y-%m")

cd "$interactions"
for f in *.jsonl; do
  [[ -e "$f" ]] || continue
  base="${f%.jsonl}"
  if [[ "$base" == "$current_month" ]]; then
    continue
  fi
  if [[ "$DRY_RUN" == "yes" ]]; then
    echo "[would gzip] $interactions/$f"
  else
    gzip -9 "$f"
    echo "[gzipped] $interactions/$f.gz"
  fi
done
