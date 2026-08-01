#!/usr/bin/env bash
# Every BR-x.y rule in docs/02-business-rules.md must be referenced somewhere in
# the Go source (a test name, a test comment, or the implementing code comment).
# A rule nobody references is a rule nobody implemented — fail the build.
#
# Usage: bash /home/dev/projects/ruuma/scripts/br-coverage.sh
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RULES_DOC="${ROOT}/docs/02-business-rules.md"

mapfile -t rules < <(grep -oE '\*\*BR-[0-9]+\.[0-9]+\.[0-9]+\*\*' "${RULES_DOC}" \
                     | tr -d '*' | sort -u)

if [ "${#rules[@]}" -eq 0 ]; then
  echo "br-coverage: no rules found in ${RULES_DOC} — check the doc format"
  exit 1
fi

missing=()
for rule in "${rules[@]}"; do
  if ! grep -rqF --include='*.go' "${rule}" "${ROOT}/internal" "${ROOT}/cmd" "${ROOT}/test" 2>/dev/null; then
    missing+=("${rule}")
  fi
done

echo "br-coverage: ${#rules[@]} rules declared, ${#missing[@]} unreferenced"

if [ "${#missing[@]}" -gt 0 ]; then
  printf '  unreferenced: %s\n' "${missing[@]}"
  exit 1
fi
