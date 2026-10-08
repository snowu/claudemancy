---
description: Draw a sigil to cast a skill
argument-hint: "[extra instructions for the skill]"
---
The claudemancy hook has just opened the sigil canvas and added context saying what the user's sigil cast: a skill to invoke, or an instruction to carry out. Follow that context exactly, including its guard against re-casting a skill that is still running.

If there is no claudemancy context in this turn, the hook didn't run. Reply with just: "✦ The spell fizzled — the claudemancy hook isn't active. Check `/hooks`."
