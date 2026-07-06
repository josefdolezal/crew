---
name: second-opinion
description: Get an independent second opinion from a different model lineage on a plan, diff, design decision, or diagnosis before committing to it. Spawns a one-shot consultant session via the crew CLI. Use when the user asks for a second opinion, a cross-model review, a sanity check of an approach, or to "ask codex/gpt what it thinks".
---

# second-opinion - one independent reviewer, different lineage

A reviewer from a different training lineage doesn't share your blind spots. This
skill spawns exactly one consultant via crew, hands it a self-contained brief,
validates what comes back, and presents a verdict. It is a review, not a fleet:
one agent, one round trip, advisory only - you stay the decision maker.

## When to use it - and when not

Use it for judgment calls where being wrong is expensive: a design about to be
built, a risky diff, a diagnosis you're about to act on, an approach the user is
unsure about.

Skip it when:
- A command can settle the question - run the tests/benchmark instead. Verification
  beats opinion whenever verification is cheap.
- The decision is trivially reversible - the round trip costs more than being wrong.
- You need a third-party fact, not a judgment - search or read the source instead.

## Picking the consultant

Lineage diversity beats raw capability: prefer a different vendor than yourself.
A same-lineage second opinion (you are Claude, consultant is Claude) still catches
mistakes but shares training blind spots - use it only when nothing else is installed.

| Consultant | Spawn | Lineage |
|---|---|---|
| Codex (GPT) | `crew spawn opinion -r codex --worktree -f brief.md` | OpenAI - first choice when you are a Claude |
| pi | `crew spawn opinion -r pi --worktree -f brief.md` | if installed |
| Claude Opus | `crew spawn opinion -r claude -m opus --worktree --yolo -f brief.md` | Anthropic - same-lineage fallback |

Check availability first (`command -v codex`, `command -v pi`). Rate what you have
on three axes - cost, intelligence (how hard a problem it handles unsupervised),
taste (code quality, API design, judgment) - against the user's actual
subscriptions, not list prices. Defaults are not limits: if the opinion you get
back is shallow, rerun once with a smarter model rather than presenting weak
findings. For anything that ships, intelligence > taste > cost.

## Flow

1. **Write the brief** to a scratch dir using
   [references/brief-template.md](references/brief-template.md). The consultant has
   no memory of your conversation - everything it needs goes in the brief. For a
   diff review, save it first: `git diff > "$DIR/diff.patch"` and reference the
   absolute path (with `--worktree` the consultant sees the last commit, not your
   uncommitted tree - the patch file is how the work under review travels).
2. **Spawn with `--worktree`** so a consultant that ignores the no-edits rule
   can't touch your tree. Then `crew wait opinion --timeout 15m --json`.
3. **Validate before presenting.** Each disagreement or finding is a claim, not a
   fact: check it against the actual code, docs, or a quick command. Cross-model
   reviews over-flag - filtering false positives is your half of the job.
4. **Present**: the consultant's verdict, which findings you confirmed vs rejected
   (and the evidence), and your own recommendation. Attribute clearly - "codex
   thinks X, I verified Y" - never launder the consultant's opinion as your own.
5. **Kill the consultant** (`crew kill opinion`). One clarifying follow-up via
   `crew send` is fine; past that, stop - two rounds of cross-model debate is the
   cap, convergence is the signal, extended argument has diminishing returns.

## Pitfalls

- A consultant verdict of "agree" is still information - report it as such; do not
  fish for objections by re-asking with a pushier brief.
- If the consultant reports `blocked` or asks for context you have, answer via
  `crew send opinion "..."` and wait again - don't respawn and lose the round.
- Don't delegate the decision. The consultant advises; you (and the user) decide.
