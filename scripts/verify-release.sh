#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: scripts/verify-release.sh X.Y.Z" >&2
  exit 1
fi

tag="v$version"
errors=0

echo "Verifying release $tag..."
echo ""

# 1. GitHub Release exists and has assets
echo "Checking GitHub Release..."
assets_count="$(gh release view "$tag" --json assets -q '.assets | length')"
if [[ "$assets_count" -eq 0 ]]; then
  echo "  FAIL: no assets found on release $tag"
  errors=$((errors + 1))
else
  echo "  OK: $assets_count assets"
fi

# 2. Release workflow succeeded
echo "Checking release workflow..."
run_conclusion="$(gh run list -L 20 --workflow release.yml \
  --json conclusion,headBranch \
  -q ".[] | select(.headBranch==\"$tag\") | .conclusion" \
  | head -n1)"
if [[ "$run_conclusion" != "success" ]]; then
  echo "  FAIL: release workflow status is '$run_conclusion'"
  errors=$((errors + 1))
else
  echo "  OK: workflow succeeded"
fi

# 3. Checksums file exists
echo "Checking checksums..."
if gh release download "$tag" --pattern "checksums.txt" --dir /tmp --clobber >/dev/null 2>&1; then
  echo "  OK: checksums.txt downloaded"
  cat /tmp/checksums.txt
  rm -f /tmp/checksums.txt
else
  echo "  FAIL: checksums.txt not found"
  errors=$((errors + 1))
fi

# 4. Released binary reports the right version
echo "Checking binary version..."
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
[[ "$arch" == "x86_64" ]] && arch="amd64"
[[ "$arch" == "aarch64" ]] && arch="arm64"
tmpdir="$(mktemp -d)"
if gh release download "$tag" --pattern "crew_${version}_${os}_${arch}.tar.gz" --dir "$tmpdir" >/dev/null 2>&1; then
  tar -xzf "$tmpdir"/crew_*.tar.gz -C "$tmpdir"
  reported="$("$tmpdir/crew" --version)"
  if [[ "$reported" == *"$version"* ]]; then
    echo "  OK: $reported"
  else
    echo "  FAIL: binary reports '$reported', expected '$version'"
    errors=$((errors + 1))
  fi
else
  echo "  SKIP: no archive for ${os}_${arch}"
fi
rm -rf "$tmpdir"

echo ""
if [[ "$errors" -gt 0 ]]; then
  echo "Release verification FAILED with $errors error(s)"
  exit 1
fi
echo "Release $tag verified."
