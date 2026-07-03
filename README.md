# crew

**One orchestrator, many coding agents.**

crew spawns interactive coding-agent sessions (Claude Code, Codex, pi, plain shells) in tmux, hands them tasks, and blocks until they report back. A human - or a stronger LLM acting as architect - delegates the boring work to cheaper models, reviews the results, and stays in control.

```bash
crew spawn worker -m haiku --worktree --yolo -t "Port src/utils to TypeScript"
crew wait worker             # blocks until the agent reports done or blocked
# worker: done (94s)
#   report: Ported 6 files, tests green
```

## Why

- **Any runtime.** Delegate to `claude`, `codex`, `pi`, or `bash` - each a full interactive CLI session, not a headless API call. Mix vendors and models per task (`-m haiku` for grunt work, anything for review).
- **Real, attachable sessions.** Agents live in tmux. `crew attach` drops you into any session to watch or take over; detach and the agent keeps working.
- **A delegation loop, not a fire-and-forget.** Spawned agents get a protocol preamble and self-report (`done` / `blocked`) when finished. `crew wait` also detects agents stuck on permission prompts (`attention`) or silently idle, so your orchestrator never hangs on a lost worker.
- **Parallel by design.** `--worktree` gives each agent its own git worktree and branch; clean worktrees vanish on kill, work-in-progress survives. Orchestrators are scoped per directory - two sessions in two repos never see each other's agents.
- **Nothing to babysit.** The daemon autostarts on first use and coordinates over a unix socket. Sessions live in tmux and state in SQLite, so a daemon restart never kills an agent.

## Quick start

Requires Go 1.24+ and tmux 3.2+.

```bash
go install github.com/josefdolezal/crew/cmd/crew@latest

crew spawn helper -t "Summarize what this repo does"   # claude is the default runtime
crew wait helper                                       # block until it reports
crew peek helper                                       # its rendered screen, anytime
crew attach helper                                     # take over (ctrl-b d to detach)
crew kill helper
```

## Command overview

| Command | Purpose |
|---|---|
| `crew spawn <name>` | Start an agent: `-r` runtime, `-m` model, `-t/-f` task, `--worktree`, `--yolo` |
| `crew wait <name>...` | Block until agents report; exit 0 only if all `done` |
| `crew send <name> <text>` | Inject a follow-up (or `--key Enter` to answer a dialog) |
| `crew inbox --drain` | Reports, agent messages, and exit events addressed to you |
| `crew list` / `peek` / `logs` / `attach` / `kill` | Observe and manage the fleet |
| `crew --version` | Print the installed crew version |

Every command takes `--json` for machine consumption - the CLI is designed to be driven by an LLM as much as by a human.

## Documentation

- [Usage guide](docs/usage.md) - all commands, flags, the delegation protocol, worktrees, identities.
- [Architecture](docs/architecture.md) - the daemon, session backends, and the design decisions behind them.
- [Agent skill](skills/SKILL.md) - drop-in skill file teaching an LLM orchestrator to drive crew.

## Status

Actively developed. The tmux backend is the only session backend today; codex/pi screen heuristics are conservative. See the [architecture notes](docs/architecture.md#roadmap) for what's next.
