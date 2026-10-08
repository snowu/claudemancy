---
description: Draw a sigil to cast a skill
argument-hint: "[extra instructions for the skill]"
---
The claudemancy hook has just opened the sigil canvas and added context naming the skill the user's sigil invoked. Follow that context exactly, including its guard against re-casting a skill that is still running: otherwise invoke that skill with the Skill tool right away.

If there is no claudemancy context in this turn, the hook didn't run. Reply with just: "✦ The spell fizzled — the claudemancy hook isn't active. Check `/hooks`."
