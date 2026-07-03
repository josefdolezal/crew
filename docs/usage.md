# Usage guide

Everything the README doesn't cover: full flags, the delegation protocol, identities, worktrees, and troubleshooting.

## Spawning agents

```bash
crew spawn <name> [flags]
```

| Flag | Meaning |
|---|---|
| `-r, --runtime` | `claude` (default) \| `codex` \| `pi` \| `bash` |
| `-m, --model` | Model passed to the runtime's native flag (e.g. `haiku`) |
| `-t, --task` / `-f, --task-file` | Initial task prompt (inline or from a file) |
| `-C, --cwd` | Working directory (default: your current dir) |
| `--worktree` | Run in a fresh git worktree of the cwd repo, branch `crew/<name>` |
| `--yolo` | Skip runtime permission prompts (claude: `--dangerously-skip-permissions`, codex: `--dangerously-bypass-approvals-and-sandbox`) |
| `--trust` | Auto-confirm startup dialogs like Claude's folder-trust prompt (default on) |
| `--attach` | Attach your terminal right after spawning |

Agent names are unique; kill an agent before reusing its name. A startup watcher confirms startup dialogs, waits for the REPL, and injects the task for runtimes that can't take it as a launch argument.

## The delegation protocol

Tasks for LLM runtimes get a preamble prepended. It tells the agent its name and to:

- run `"$CREW_BIN" report --status done -m "<summary>"` when finished,
- run `"$CREW_BIN" report --status blocked -m "<what it needs>"` when stuck,
- message its orchestrator mid-task with `"$CREW_BIN" send parent "<text>"`,
- never exit its own session.

`crew wait <name>... [--timeout 10m] [--for done|ready]` long-polls the daemon until each agent resolves:

| Outcome | Meaning | What to do |
|---|---|---|
| `done` / `blocked` | Agent reported; report attached | Read it, act |
| `attention` | Agent is stuck on a permission/confirmation dialog (screen tail attached) | `crew send <name> '1'` or `crew send <name> --key Enter`, then wait again |
| `idle` | Output quiet 30s + screen looks like a waiting prompt, but no report | `crew peek <name>`, nudge with `crew send` |
| `exited` | Runtime process ended without reporting | `crew logs <name>` for the post-mortem |
| `ready` | Only with `--for ready`: REPL is up | Start interacting |
| `timeout` | `--timeout` elapsed | Investigate with `peek` |

Exit code is 0 only when every named agent reported `done`, so `crew wait a b c && next-step` works in scripts.

## Messaging

```bash
crew send worker "also add tests"     # into an agent's session, signed [<your-identity>]
crew send worker --key Enter          # a named key (Enter, Escape, Up, ...) for dialogs
crew send parent "need a decision"    # from inside an agent: to the orchestrator's inbox
crew inbox                            # unread messages: reports, agent messages, exit events
crew inbox --drain                    # ...and mark them read
crew inbox --all                      # include already-read
```

Messages to agents land on their stdin as if typed. Messages to non-agent identities (your orchestrator) land in an inbox. A watchdog also posts inbox events when an agent's process dies or hits a permission prompt.

## Identity

Every spawn records who spawned it; `list`, `inbox`, and report routing are scoped by that identity:

1. `CREW_AGENT_NAME` - set automatically inside agent sessions.
2. `CREW_IDENTITY` - set it yourself to pin a stable identity for a long-lived orchestrator session (recommended if your session may change cwd).
3. `orchestrator@<cwd>` - the default: sessions in different directories are distinct orchestrators automatically.

`crew list --all` crosses identity boundaries when you need the full picture.

## Worktrees

`crew spawn <name> --worktree` creates `~/.crew/worktrees/<name>` on branch `crew/<name>` from the repository you're in. On `crew kill`:

- clean worktree: removed, and the branch deleted if fully merged;
- uncommitted changes or unmerged commits: both kept, and the kill output tells you where.

## Observing

```bash
crew list [--all] [--json]     # NAME RUNTIME MODEL STATUS AGE CWD PARENT
crew peek <name>               # rendered screen (what a human would see)
crew logs <name> -n 200        # raw output log, ANSI-stripped (--raw to keep escapes)
crew attach <name>             # full tmux attach; detach with ctrl-b d
crew --version                 # installed crew version
```

`status` is computed live from tmux: `running`, `exited` (process ended, screen still inspectable), or `gone` (session vanished).

## The daemon

Autostarts on first use; manual control via `crew daemon run|stop|status`. Stopping the daemon never touches agent sessions - they live in tmux, and a restarted daemon re-adopts them from its SQLite registry.

State lives under `~/.crew` (override with `CREW_HOME`): `crew.sock` (unix socket, mode 0600 - filesystem permissions are the auth boundary; single host only), `crew.db`, `logs/<agent>.log`, `worktrees/`.

## Environment inside agent sessions

| Var | Meaning |
|---|---|
| `CREW_AGENT_NAME` | The agent's identity for `report` / `send` |
| `CREW_PARENT` | The orchestrator identity that spawned it |
| `CREW_BIN` | Absolute path to the crew binary |
| `CREW_HOME` | State directory (shared with the daemon) |

## Troubleshooting

- **Agent stuck at a folder-trust / theme dialog**: shouldn't happen with `--trust` (default); if it does, `crew send <name> --key Enter`.
- **`agent already exists`**: `crew kill <name>` first; reports of the old agent are cleared when the name is respawned, so drain your inbox before reusing names.
- **`gone` agents after a reboot**: tmux sessions don't survive reboots; kill the stale entries.
- **Empty inbox when you expected messages**: check your identity - are you in the same directory the spawn ran from (or using the same `CREW_IDENTITY`)? `crew list --all` shows which parent each agent reports to.
