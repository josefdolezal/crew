---
name: llm-council
description: Convene a council of diverse LLMs on a hard question - independent answers, anonymized peer review, chairman synthesis - via the crew CLI. Use when the user asks for a council, wants multiple models to weigh in on a design decision, architecture choice, or ambiguous tradeoff, or when a single model's blind spot is the main risk.
---

# llm-council - independent answers, anonymized review, one synthesis

Karpathy's LLM-council pattern on crew sessions: N models answer the same question
independently, then rank each other's anonymized answers, then a chairman merges
the strongest points into one response. The anonymization is load-bearing - models
can't play favorites when they don't know whose answer they're grading. You are
the chairman, never a council member: if your own answer sat in the pile, your
synthesis would favor it.

## When a council is worth N models

Councils are for judgment questions where being wrong is expensive and no command
can settle it: architecture choices, migration strategies, API design, build-vs-buy.
The cost is deliberate - N answers plus N reviews. Skip it when:

- verification is cheap: run the test/benchmark, don't convene a vote on it
- the question has a knowable answer: search or read the source
- a single cross-check would do: use the `second-opinion` skill instead

## Roster

3-4 members, maximum lineage diversity - the council's value is uncorrelated blind
spots, so never seat two copies of the same model if anything else is installed
(`command -v codex`, `command -v pi`). Rate candidates on cost / intelligence /
taste against the user's actual subscriptions; for a council, intelligence
dominates - a weak member costs a seat and adds noise.

| Member | Spawn (stage 1) |
|---|---|
| Codex (GPT) | `crew spawn member-codex -r codex --worktree -f "$COUNCIL/task-codex.md"` |
| Claude Opus | `crew spawn member-opus -r claude -m opus --worktree --yolo -f "$COUNCIL/task-opus.md"` |
| Claude Sonnet | `crew spawn member-sonnet -r claude -m sonnet --worktree --yolo -f "$COUNCIL/task-sonnet.md"` |
| pi | `crew spawn member-pi -r pi --worktree -f "$COUNCIL/task-pi.md"` |

## Stage 1 - independent answers

1. `COUNCIL=$(mktemp -d "$SCRATCHPAD/council-XXXXXX")` and `mkdir -p "$COUNCIL"/{answers,anon,reviews}`.
2. Write one brief per [references/council-brief-template.md](references/council-brief-template.md).
   Members have no memory of your conversation; the brief stands alone.
3. Per member, write a task file: the shared brief plus one line - "Write your full
   answer to `$COUNCIL/answers/<member>.md`, then report done with a one-line
   position summary." (Answers travel by file; report bodies are for summaries.)
4. Spawn all members in parallel, then `crew wait member-... --timeout 20m --json`
   with every name. An `exited` member is dropped from the council, not retried -
   note the missing seat in the final report.

Members must not see each other's answers in this stage - independence is the point.

## Stage 2 - anonymized peer review

1. Shuffle-copy answers to `$COUNCIL/anon/response-A.md`, `-B.md`, ... and record
   the mapping yourself. Never reveal it to reviewers.
2. For each member model, spawn a FRESH session (fresh context - the authoring
   session knows its own answer) with a review task per
   [references/review-template.md](references/review-template.md): evaluate every
   anonymized response on accuracy and insight, identify concrete errors, produce a
   strict ranking, write it to `$COUNCIL/reviews/<member>.md`.
3. Caveat to keep in mind, not to fix: anonymization removes brand bias, but a
   model may still recognize its own prose. Treat self-rankings as slightly inflated.

## Stage 3 - chairman synthesis (you)

Read all answers and reviews, then:

1. **Tally rankings** with the mapping restored. Consensus top answers and
   consistently-flagged errors are the strong signals.
2. **Validate before adopting**: a point every reviewer praised can still be wrong -
   check load-bearing claims against the code/docs like any cross-model finding.
3. **Synthesize** one recommendation: lead with the answer, then where the council
   agreed (strong signal), where it split (name the sides and their reasons, then
   make your call), and any error a reviewer caught that changed the outcome.
4. Report splits honestly. A 2-2 council is a finding in itself - do not
   manufacture consensus the council didn't reach.

Then `crew kill` all members and reviewers. Keep `$COUNCIL` as the record; link
the user to it. One round only - no debate loops, no re-votes; if the synthesis
surfaces a genuinely new question, that's a new council (ask the user first).
