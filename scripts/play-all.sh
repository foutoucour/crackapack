#!/usr/bin/env bash
# Serve the web client on every set in scope (ADR-0039), opened in stages (ADR-0053, ADR-0054).
# Do not pass --sets: a game played from a list of sets has no stage and every set is open.
#
# Usage: scripts/play-all.sh [extra play flags, e.g. --seed 42 --save ./my-save.json]
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
catalogue_dir="$repo_root/catalogues"
prices="$repo_root/prices.json"

for path in "$catalogue_dir" "$prices"; do
  if [[ ! -e "$path" ]]; then
    echo "missing $path, download the data with: go run ./tools/fetchdata --sets <codes>" >&2
    exit 1
  fi
done

cd "$repo_root"
exec go run ./cmd/crackapack play \
  --catalogue-dir "$catalogue_dir" \
  --prices "$prices" \
  "$@"
