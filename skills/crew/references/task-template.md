# Task file template

For non-trivial delegations, structure the task file (`-f task.md`) with these
blocks. A spawned worker has no memory of your conversation and follows explicit
contracts far better than prose. Delete a block only if it is genuinely empty for
this task - an omitted `<done_when>` or `<files>` is how a worker ends up guessing.

```xml
<task>
What to build/change, in 2-6 sentences. Name the repo area it lives in and any
context a fresh engineer would need. State the WHY in one line if it affects
implementation choices.
</task>

<done_when>
- Checkable criterion, e.g. "go test ./... passes, including new tests covering X"
- Checkable criterion, e.g. "gofmt -l . reports nothing"
</done_when>

<files>
Touch:
- path/to/file.go - what changes here
- path/to/new_file.go - create; what goes here

Do NOT touch:
- anything else, especially: config, CI, unrelated modules, lockfiles unless required
</files>

<interfaces>
Exact signatures, types, or API shapes other code depends on. Paste real code, not prose.
</interfaces>

<constraints>
- Match the existing style and patterns of the surrounding code.
- No new dependencies unless listed here: [none / list]
- Project-specific hard rules.
</constraints>

<verification>
Run these before reporting done and include their real output in your report:
- command 1, e.g. go test ./...
- command 2, e.g. go vet ./...
</verification>

<report>
Structure your done report body as three sections:
1. CHANGED - file-by-file summary of what you did.
2. VERIFIED - each verification command with its actual output (trimmed to the
   relevant lines). If a command failed, say so plainly; do not claim success.
3. OPEN - anything left undone, skipped, decided, or discovered along the way, and why.
</report>

<follow_through>
Work until done_when is satisfied. Decide routine questions yourself and record
the choice under OPEN. Report blocked only when the task is impossible as
specified - say exactly what is missing.
</follow_through>

<action_safety>
Stay narrow. No refactors, cleanups, or "improvements" outside the files listed
above. Do not delete or rewrite code you do not understand - flag it under OPEN.
</action_safety>
```

The `<report>` block shapes the report *content* only - how to report (the `crew
report` mechanics) comes from the protocol preamble; do not restate it in the task.
