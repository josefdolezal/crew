# Council review template

For stage-2 reviewers: fresh sessions, one per member model. Substitute the
anonymized response paths and the output path.

```xml
<task>
You are reviewing N anonymized answers to the same question. You do not know which
model wrote which answer - judge the text, not a reputation. Read the original
brief at [absolute path], then every response: [absolute paths to response-A.md,
response-B.md, ...].
</task>

<evaluation>
Score each response on two axes:
- ACCURACY - are the technical claims correct? Check the checkable ones against
  the repo/docs (read-only commands are fine). A confident wrong claim is worse
  than a hedged right one.
- INSIGHT - does it answer the real question and the stated success criteria, or
  a nearby easier one? Does it surface a consideration the others missed?
</evaluation>

<output>
Write to [absolute output path]:
1. PER-RESPONSE - for each response: 2-4 sentences on its strengths, plus any
   concrete error with the evidence (quote the claim, state why it is wrong).
2. RANKING - strict order, best first, one-line justification per rank. No ties.
3. CONSENSUS - points where multiple responses independently agree (that
   independence makes them the strongest signal in the council).
Then report done with your ranking as a one-line summary (e.g. "B > A > C").
</output>

<boundaries>
Do not write your own answer to the question - rank what exists. Make no edits
outside the output file.
</boundaries>
```
