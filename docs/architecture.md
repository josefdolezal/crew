# Architecture

```
crew spawn/send/wait/...  ──HTTP over unix socket──▶  crew daemon
                                                        ├─ registry (SQLite ~/.crew/crew.db)
                                                        ├─ session backend (tmux)
                                                        ├─ startup watcher (dialogs, REPL-ready, task injection)
                                                        ├─ watchdog (exit + attention events → inboxes)
                                                        └─ raw output logs (~/.crew/logs/)
```

A single Go binary. `crew daemon run` serves; every other subcommand is a thin client that autostarts the daemon when needed.

## Design decisions

**Sessions live in tmux, not in the daemon.** The daemon could own PTYs directly, but then a daemon crash would kill every agent. Instead agents run as windows of a single `crew` tmux session (identifier `crew:<name>`), so anyone attached sees the whole fleet in the window list; the daemon coordinates (registry, routing, waiting, watching) and re-adopts running sessions from SQLite after a restart. Killing the daemon is always safe. Plain session names remain valid backend identifiers - adopted orchestrator sessions and pre-window agents resolve through the same code path.

**Reads are rendered screens, not raw streams.** Agent CLIs are TUIs; their raw output is cursor-movement soup. tmux maintains the virtual screen for us - `crew peek` returns `capture-pane` output, which is what a human would see. Raw streams are still logged per agent (`pipe-pane`) for forensics.

**Completion is a protocol, with heuristic fallbacks.** The daemon can't know when a foreign CLI "finished", so agents self-report through a preamble contract (`crew report`). Screen heuristics back it up: per-runtime patterns detect permission dialogs (`attention`) and idle prompts (`idle`), with output-quiescence as the generic fallback for runtimes whose TUI patterns aren't pinned down.

**No singleton orchestrator.** Identity is `CREW_AGENT_NAME` (agents) / `CREW_IDENTITY` (pinned) / `orchestrator@<canonical cwd>#<tmux pane>` (default; the pane part is dropped outside tmux). The canonical cwd comes from the kernel, not `$PWD`, because shells can report case/symlink variants of the same directory. The pane id keeps two orchestrator sessions in the same directory distinct - cwd alone would merge their identities and route both sets of reports to one inbox. For the same reason push delivery adopts the pane, not the tmux session: injecting into a session lands on its *active* window, the wrong orchestrator when several run as windows of one session. Nesting (agents spawning agents) falls out of the same parent field.

**Unix socket, mode 0600.** Filesystem permissions are the auth boundary. No TCP port, no tokens, single host by design. Do not expose the socket.

## Packages

| Package | Role |
|---|---|
| `internal/proto` | Request/response types shared by client and daemon |
| `internal/daemon` | HTTP server, spawn/kill, wait state machine, startup watcher, watchdog |
| `internal/backend` | `Backend` interface + tmux implementation (spawn, send-keys, capture-pane, pipe-pane) |
| `internal/runtime` | Per-runtime adapters: launch command, task passing, screen probes (startup/idle/attention) |
| `internal/store` | SQLite registry: agents + messages (reports, routed messages, events) |
| `internal/client` | Unix-socket HTTP client with daemon autostart |
| `internal/cli` | Cobra command tree, `--json` everywhere |
| `internal/gitx` | Worktree create/remove-if-clean |

Adding a runtime is one file in `internal/runtime` (command builder + screen patterns) plus a `Lookup` case.

## Backends

`internal/backend.Backend` abstracts the session substrate:

```go
Spawn, SendInput, SendKey, Snapshot, ActivityAt, State, Kill, AttachArgs
```

tmux (>= 3.2) is the only implementation. A daemon-owned PTY backend (creack/pty + an embedded VT emulator, attach bridged over the socket) fits behind the same interface if tmux ever becomes the limiting factor - it hasn't yet, and the tmux property "sessions outlive the daemon" is worth keeping.

## Roadmap

- Integration tests for the daemon's wait state machine against a fake backend.
- Pinned codex/pi screen patterns (startup, idle, approval prompts) - currently conservative quiescence-based fallbacks.
- `crew purge` for `gone` registry rows (e.g. after a reboot), broadcast send, log rotation.
- Optional internal PTY backend behind `--backend`.
