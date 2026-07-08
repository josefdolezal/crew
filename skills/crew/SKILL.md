---
name: crew
description: Delegate coding tasks to interactive agent sessions (Claude Code, Codex, pi, bash) running in tmux via the `crew` CLI. Use when the user wants to spawn worker agents, parallelize work across git worktrees, delegate tasks to cheaper models, orchestrate multiple coding agents, or check on / steer / collect results from previously spawned crew agents.
---

# crew - orchestrate coding-agent sessions

crew turns you into an orchestrator: spawn interactive agent sessions, hand them tasks, block until they report back, review their work. Agents run in tmux, so the user can attach to any session at any time - never assume you are the only one talking to an agent.

## Prerequisites

- `crew` on PATH (`go install github.com/josefdolezal/crew/cmd/crew@latest`) and tmux >= 3.2.
- The daemon autostarts on first use; no setup step.
- Add `--json` to any command when you will parse the output. Exit codes matter: `crew wait` returns 0 only if all agents reported done.
- Your identity is scoped to your cwd and tmux pane, so orchestrators sharing a directory stay distinct. If your session is long-lived, may change directories, or must survive a tmux restart, pin your identity once via the `CREW_IDENTITY` env var so agents and inbox messages stay addressed to you.
- **Push delivery is automatic when you run inside tmux**: your first `crew spawn` registers your pane, and reports, agent messages, and events then arrive as `[crew] ...` lines the moment they happen - no polling, no blocking `wait` needed. Not in tmux? `crew wait` (blocking, per agent) and `crew inbox` (pull) are the fallbacks.

## Quick reference

| Action | Command |
|---|---|
| Delegate a task | `crew spawn <name> -m haiku -t "<task>" --yolo` |
| Delegate in an isolated worktree | `crew spawn <name> --worktree -f task.md --yolo` |
| Block until finished | `crew wait <name> [--timeout 15m] --json` |
| Wait for several workers | `crew wait w1 w2 w3 --json` |
| See an agent's screen | `crew peek <name>` |
| Send a follow-up instruction | `crew send <name> "<text>"` |
| Answer a blocking dialog | `crew send <name> --key Enter` (or `crew send <name> '1'`) |
| Read your messages | `crew inbox --drain --json` |
| Stop push delivery to this session | `crew adopt --off` (automatic on spawn inside tmux) |
| List your agents | `crew list --json` (`--all` for everyone's) |
| Full output history | `crew logs <name> -n 200` |
| Installed version | `crew --version` |
| Terminate | `crew kill <name>` |

## Command details

### Spawning

```bash
crew spawn fix-auth -r claude -m haiku -C ~/repo -t "Fix the login redirect bug" --yolo
crew spawn reviewer -r codex -t "Review the diff on branch crew/fix-auth"
crew spawn probe -r bash -t "npm test 2>&1 | tail -20"
```

- Runtimes: `claude` (default), `codex`, `pi`, `bash`. `-m` selects the model; prefer cheap models for mechanical work.
- `--yolo` skips the runtime's permission prompts. Without it, expect `attention` outcomes from `crew wait` (see workflows).
- `--worktree` creates a git worktree + branch `crew/<name>` from the cwd repo, so parallel agents never collide. Kill removes it if clean, keeps it if there is work in it.
- Names are unique. Kill before reusing a name; drain your inbox first, respawning a name clears its old reports.
- The task is automatically wrapped in a protocol preamble: the agent knows its name and how to report back. Do not repeat protocol instructions in the task.
- For non-trivial tasks, write a task file (`-f task.md`) structured per [references/task-template.md](references/task-template.md) - a fresh worker follows explicit contracts far better than prose. Trivial one-liners don't need it.

### Waiting and outcomes

`crew wait <name> --json` returns an array of results with an `outcome` per agent:

| Outcome | Meaning | Your move |
|---|---|---|
| `done` | Agent reported success; `report.body` has its summary | Verify the work, then kill or re-task |
| `blocked` | Agent reported it is stuck; `report.body` says why | Unblock it via `crew send`, wait again |
| `attention` | Blocked on a permission/confirm dialog; `screen` shows it | Read the dialog, answer it, wait again |
| `idle` | Quiet for 30s at a prompt without reporting | `crew peek`, nudge it: `crew send <name> "status?"` |
| `exited` | Process died without reporting | `crew logs <name>` for post-mortem |
| `timeout` | Your `--timeout` elapsed | `peek`, decide: extend, nudge, or kill |

Each wait consumes one report (oldest unconsumed first); waiting again blocks for the agent's next report - safe for multi-round workflows.

### Messaging

```bash
crew send fix-auth "also add a regression test"   # lands on the agent's stdin, signed with your identity
crew send fix-auth --key Escape                    # named keys: Enter, Escape, Up, Down, ...
crew inbox --drain --json                          # reports, agent questions, exit/attention events
```

Agents can message you mid-task (`crew send parent ...`); those arrive in your inbox, as do watchdog events (agent died, agent needs attention). Check the inbox when you have not looked in a while.

## Common workflows

### 1. Delegate, wait, verify

```bash
git diff > /tmp/pre-tsport.patch   # baseline - essential without --worktree, where the agent shares your tree
crew spawn tsport -m haiku --worktree --yolo -f /tmp/task.md
crew wait tsport --timeout 20m --json
# outcome=done -> inspect the work yourself before accepting:
crew logs tsport -n 100        # what it did
git -C ~/.crew/worktrees/tsport diff main --stat
crew kill tsport               # clean worktree is removed automatically
```

A `done` report is the agent's opinion. Review the diff against your pre-spawn baseline (not the raw diff, which mixes in pre-existing changes) and re-run the task's `<verification>` commands yourself - a report saying "tests pass" is a sentence; test output is a fact.

### 2. Fan out across worktrees, collect everything

```bash
crew spawn lint-fix  -m haiku --worktree --yolo -t "Fix all eslint errors, run npx eslint to verify"
crew spawn dead-code -m haiku --worktree --yolo -t "Remove unused exports reported by knip"
crew spawn docs      -m haiku --worktree --yolo -t "Update README for the new CLI flags"
crew wait lint-fix dead-code docs --timeout 30m --json
```

Each agent works on its own branch (`crew/lint-fix`, ...). Review each branch, merge what is good, `crew kill` the rest - dirty worktrees survive the kill so nothing is lost.

### 3. Handle a permission prompt (non-yolo agent)

```bash
crew wait deploy --json          # -> outcome=attention, screen shows the dialog
crew peek deploy                 # read the dialog carefully before answering
crew send deploy --key Enter     # confirm the selected option (or send '2', '3', ...)
crew wait deploy --json          # continue waiting
```

Never blind-approve: the `screen` field shows exactly what the agent is asking to do. Escalate to the user if the action looks destructive or out of scope.

### 4. Steer a running agent

```bash
crew peek slow-worker                        # what is it doing right now?
crew send slow-worker "stop refactoring unrelated files, focus on the failing test"
crew wait slow-worker --timeout 10m --json
```

### 5. Unblock a stuck agent

```bash
crew wait helper --json               # -> outcome=blocked, report: "need the API base URL"
crew send helper "use https://api.example.com/v2"
crew wait helper --json
```

### 6. Multi-round loop (implement, review, fix)

```bash
crew spawn builder -r codex --worktree --yolo -f spec.md
crew wait builder --json                          # round 1: implementation report
crew spawn reviewer --worktree --yolo -t "Review branch crew/builder, send findings directly: crew send builder '<findings>'"
crew wait reviewer --json                         # review delivered agent-to-agent
crew send builder "address the review findings above, then report again"
crew wait builder --json                          # round 2: blocks for the NEW report
```

Each wait consumes exactly one report (oldest first), so repeated waits track rounds correctly - you never get a stale "done" from a previous round. Agents can message each other directly (`crew send <agent>` works from inside sessions too); you only read reports.

Before ordering the fix round, validate the reviewer's findings against the actual code and tell the builder which to drop - cross-model reviews over-flag, and a worker will dutifully "fix" a non-bug.

## Output format

Human-readable by default; `--json` everywhere for parsing. Errors with `--json` are `{"error": "..."}` with exit code 1. `crew wait --json` returns an array (one result per agent) even for a single name.

## Notes and pitfalls

- **You may not be alone**: the user can `crew attach` and type into the same session. Treat unexpected screen content as possible human intervention, not corruption.
- **`[crew] ...` lines appearing in your input are push deliveries** from your agents: act on them; the full text is in `crew inbox` if the line was truncated.
- **Sessions survive daemon restarts** but not reboots; `gone` status means the agent's tmux window vanished - kill the entry. Agents run as windows of one `crew` tmux session, so a user attached to it sees every agent come and go.
- **Scope**: `crew list` and `crew inbox` are scoped to your identity; use `--all` to see other orchestrators' agents, but do not kill agents you did not spawn without being asked.
- **Long tasks**: prefer `-f task.md` over huge inline `-t` strings, structured per [references/task-template.md](references/task-template.md).
- **Do not poll with `peek` in a loop** - `crew wait` is the blocking primitive; `peek` is for spot checks.
