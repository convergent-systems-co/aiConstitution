#!/usr/bin/env bash
# scripts/release.sh — cut a release.
# Usage: scripts/release.sh vX.Y.Z
set -euo pipefail
[ $# -eq 1 ] || { echo "Usage: $0 vX.Y.Z" >&2; exit 1; }
version="$1"
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Bad version: $version" >&2; exit 1; }

git diff-index --quiet HEAD -- || { echo "Working tree dirty" >&2; exit 1; }
git tag -a "$version" -m "$version"
git push origin "$version"
echo "Tag pushed. Release workflow will run."
