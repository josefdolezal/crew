#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  echo "usage: scripts/release.sh X.Y.Z" >&2
  exit 1
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# Guard: must be on main
branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$branch" != "main" ]]; then
  echo "error: must be on main (currently on $branch)" >&2
  exit 1
fi

# Guard: clean working tree
if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree not clean" >&2
  exit 1
fi

# Guard: build, vet, tests pass
echo "Running checks..."
go build ./...
go vet ./...
go test ./...

# Create and push tag
tag="v$version"
if git rev-parse "$tag" >/dev/null 2>&1; then
  echo "Tag $tag already exists — skipping tag creation"
else
  git tag -a "$tag" -m "Release $version"
  echo "Created tag $tag"
fi

git push origin main --tags
echo ""
echo "Pushed $tag to origin. GitHub Actions will:"
echo "  1. Build darwin/linux binaries (amd64 + arm64)"
echo "  2. Create the GitHub Release with archives and checksums"
echo ""
echo "Next steps:"
echo "  - Watch the release workflow: gh run watch"
echo "  - Verify: scripts/verify-release.sh $version"
