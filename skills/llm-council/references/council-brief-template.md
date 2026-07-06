# Council brief template

One brief, identical for every member - fairness requires the same inputs. Members
are fresh sessions with no memory of your conversation; the brief stands alone.

```xml
<question>
The decision to make, as one sentence, then the options already on the table (if
any). Do not signal which option you or the user prefers - it biases the council.
</question>

<context>
What is being built, the repo area, and the forces that matter: scale, team size,
deadlines, compatibility constraints, what already exists. 5-10 sentences. Include
absolute paths to key files if repo inspection would sharpen the answer.
</context>

<success_criteria>
What a winning answer optimizes for, in priority order - e.g. "1. operational
simplicity, 2. migration risk, 3. cost". Without this, members optimize for
different things and the rankings measure taste, not quality.
</success_criteria>

<output>
Write your answer to the file path given in your task, structured as:
1. POSITION - your recommendation in one paragraph, stated first.
2. REASONING - why, grounded in the context and criteria above.
3. ALTERNATIVES - the strongest option you rejected and the single best argument
   for it (steelman, don't strawman).
4. CONFIDENCE - high / medium / low, plus what evidence would change your mind.
Then report done with a one-line summary of your position.
</output>

<boundaries>
Answer independently - do not look for other members' answers. Read-only commands
are fine (grep, build, test); make no edits.
</boundaries>
```
