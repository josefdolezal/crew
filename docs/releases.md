# Releases

Releases are cut from `main` with an annotated tag; GitHub Actions runs [GoReleaser](https://goreleaser.com) and publishes the artifacts. Versioning is semver with a `v` prefix (`v0.2.0`).

## Cutting a release

```bash
scripts/release.sh 0.2.0
```

The script refuses to run unless you are on `main` with a clean working tree, and `go build` / `go vet` / `go test` all pass. It then creates the annotated tag `v0.2.0`, pushes `main` with tags, and the [release workflow](../.github/workflows/release.yml) takes over:

1. builds `crew` for darwin/linux, amd64 + arm64 (`CGO_ENABLED=0`, `-trimpath`),
2. stamps the binary via ldflags (`crew --version` reports `0.2.0 (<commit>, <date>)`),
3. publishes a GitHub Release with `crew_<version>_<os>_<arch>.tar.gz` archives (each bundling README, LICENSE, docs) and a `checksums.txt`.

## Verifying

```bash
gh run watch                     # follow the workflow
scripts/verify-release.sh 0.2.0  # assets present, workflow green, checksums, binary reports the right version
```

## Re-releasing a tag

If a release run failed for infrastructure reasons - or the GoReleaser config itself needed a fix - re-run without moving the tag: GitHub → Actions → Release → *Run workflow*, enter the tag. The workflow checks out the tag's tree but uses the **current** `main` GoReleaser config (stashed before checkout), so config fixes apply without retagging.

## Local dry run

```bash
goreleaser check                      # validate .goreleaser.yaml
goreleaser release --snapshot --clean # full build into dist/, no publish
```

Snapshot builds are versioned `<next-patch>-next` and never touch GitHub.

## What is deliberately not here

- **Homebrew tap**: not set up yet. When wanted, add a `brews:` section pointing at a `josefdolezal/homebrew-tap` repository plus a `HOMEBREW_TAP_TOKEN` secret.
- **Windows builds**: crew shells out to tmux, which has no native Windows port; WSL users are covered by the linux binaries.
