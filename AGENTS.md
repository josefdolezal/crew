# Agent instructions for crew

Go CLI + daemon that orchestrates interactive coding-agent sessions (claude, codex, pi, bash) in tmux. Architecture and design rationale: [docs/architecture.md](docs/architecture.md). Command reference: [docs/usage.md](docs/usage.md). Do not restate their content in answers; link or quote the relevant line.

## Interaction style

- Be direct. Lead with the answer or the change; skip preambles, recaps of the question, and closing summaries of what you just said.
- Don't ask for confirmation you don't need. When the request is unambiguous and the change is contained, do it. Report the result, not your plan to do it.
- When genuinely ambiguous (multiple valid designs, destructive action, unclear scope), ask once with 2-3 concrete options and a recommended default. Never a bare open-ended question.
- Push back when an instruction conflicts with the architecture decisions in docs/architecture.md; propose the alternative in one line, then do what the user decides.
- No flattery, no hedging, no restating tradeoffs both ways without a recommendation.
- Token efficiency beats ceremony: no boilerplate comments, no redundant tests of the same path, no explanatory prose inside code that the code already says.

## Build, test, verify

```bash
go build ./... && go vet ./... && gofmt -l .   # must all be clean
go test ./...
```

For behavior changes, verify end-to-end against real tmux sessions - unit tests don't cover the daemon:

```bash
go build -o /tmp/crew-dev/crew ./cmd/crew
/tmp/crew-dev/crew daemon stop   # next command autostarts the rebuilt binary
/tmp/crew-dev/crew spawn t1 -r bash --cwd /tmp -t 'echo hi'   # cheap runtime for plumbing tests
# ... exercise the change (wait/peek/send/inbox), then:
/tmp/crew-dev/crew kill t1 && /tmp/crew-dev/crew daemon stop
```

Use `-r bash` for lifecycle/plumbing verification; only spawn `-r claude -m haiku` when the change touches LLM-specific behavior (preamble, attention, idle detection) - it costs tokens.

## Release

Once a task is done and committed to main, cut a release: `scripts/release.sh X.Y.Z`. It re-runs the checks, tags, and pushes; CI builds the binaries and publishes the GitHub Release + Homebrew formula. Patch bump for fixes/docs/skill changes, minor for new commands or behavior changes. Confirm the workflow succeeded (`gh run watch`) or verify with `scripts/verify-release.sh X.Y.Z`.

## Conventions

- Pure Go, no CGo (modernc.org/sqlite). Don't add dependencies without stating why in the PR/commit.
- New runtime = one file in `internal/runtime` (command builder + screen probes) + a `Lookup` case. Screen-pattern heuristics must fail toward `StartupBooting` / `""` (unsure), never toward false positives.
- Client and daemon share types only through `internal/proto`. The daemon never imports `internal/cli` or `internal/client`.
- Status is computed live from the backend; never persist derived state that can go stale.
- Every command supports `--json`; new commands must too, including their error paths.
- User-visible behavior changes require updating README.md, docs/usage.md, and skills/SKILL.md in the same change - they drift otherwise.
- gofmt formatting, comments only where the code can't say it (constraints, protocol contracts, non-obvious failure modes).
