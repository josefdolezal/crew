# Second-opinion brief template

The consultant is a fresh session with no memory of your conversation. The brief
must stand alone. Delete a block only if genuinely empty.

```xml
<context>
What is being built/changed and why, in 2-5 sentences. Repo area, relevant
constraints (deadlines, compatibility, conventions). Enough for a senior engineer
who just walked in.
</context>

<artifact>
The thing under review, verbatim or by absolute path:
- a plan/design: paste it here
- a diff: absolute path to the .patch file (you are in a worktree at the last
  commit; the patch is the work under review)
- a diagnosis: the symptom, the evidence, and the proposed cause
</artifact>

<question>
The specific asks, numbered. "Is this sound?" is weak; "Does the migration order
in step 3 survive a mid-deploy crash?" is strong. Include the decision you are
leaning toward so the consultant reviews a position, not a vacuum.
</question>

<calibration>
- If the approach is sound, say so plainly. Do not invent objections to appear
  thorough - "no significant issues" is a valid, useful verdict.
- Rate each finding: severity (blocker / significant / minor) and your confidence
  (high / medium / low). Scale severity to this project, not to an imagined
  mission-critical system.
- Ground every finding in the artifact - quote the line or step you object to.
</calibration>

<output>
Structure your report as:
1. VERDICT - one of: agree / agree with changes / disagree, plus a one-line reason.
2. FINDINGS - numbered, each with severity, confidence, and the evidence.
3. ALTERNATIVE - only if you disagree: the approach you would take and why it wins.
</output>

<boundaries>
Review only - make no edits, run read-only commands only (build/test/grep are fine).
Critique the artifact, not the surrounding codebase.
</boundaries>
```
