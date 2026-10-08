---
name: inscribe
description: Add a new claudemancy spell, or change, override or disable an existing one. A spell is a sigil drawing that casts a skill or a prompt when the user draws it after /cast. Use when the user wants a spell for a skill or an instruction, a different drawing or symbol for a spell, or to customise or remove a default spell.
argument-hint: "[what the spell should cast, and optionally a symbol]"
---

# Inscribe a claudemancy spell

A spell is one plain-text `.sigil` file: a header, a `---` line, then an ASCII drawing of the sigil. Any character other than a space or `.` is ink.

```
name: Ward of Deployment
skill: deploy
args: staging
---
           #
         #   #
       #       #
     #           #
   #               #
     #           #
       #       #
         #   #
           #
```

The tools live in this plugin. The plugin root is two directories above this SKILL.md:

```sh
CLI="<plugin root>/bin/claudemancy-cli"
"$CLI" --list --project "$PWD"          # existing spells, their files, the spell folders
"$CLI" --suggest 5 --project "$PWD"     # most distinct free symbols, drawn and ready to paste
"$CLI" --check --project "$PWD" [draft] # validate everything; flags look-alike sigils
```

## Workflow

1. **Work out what the spell casts.** Ask only if it isn't clear.
   - A skill: `skill: <name>`, plus optional `args:`. Use the exact skill name, without the slash, and with the plugin prefix if it has one (`my-plugin:deploy`).
   - Anything else: `prompt: <instruction>`. Write it as the user would type it. Make it self-contained and safe to run whenever cast.
2. **Pick the folder.**
   - User folder `~/.config/claudemancy/spells/` by default.
   - Project folder `<repo>/.claude/claudemancy/spells/` when the spell is for this repo or the team.
   - Never edit the plugin's built-in `spells/` in an installed plugin.
3. **Look at what exists:** run `--list`. If the user wants to change an existing spell, note its file name. The **same file name** in your folder overrides it.
4. **Choose the symbol.** It must be drawable in one stroke, without lifting the mouse, and easy to draw quickly.
   - If the user named a shape, draw it.
   - Otherwise run `--suggest 5` and offer the top two or three, showing their drawings, or take the first if the user doesn't mind.
   - Suggestions are already checked: they are one stroke, free, and recognised in at least 80% of simulated hand-drawn casts.
   - Add a second or third drawing for the obvious ways people vary the shape (wider, narrower, rounder). One drawing at one proportion makes sloppy versions fizzle.
5. **Write the file** as `<folder>/<id>.sigil`, where the id uses lowercase letters, digits and dashes (e.g. `deploy-staging`). Give it an evocative `name:`; the defaults use Doctor Strange-style names like "Ward of Cyttorak".
6. **Run `--check`.**
   - Fix every `✗` (a broken file, or a sigil too similar to another) before finishing.
   - A `⚠` (close pair) is acceptable; mention it.
7. **Tell the user:** the file path, what to draw, and that `/cast` will now offer it. Suggest they train their own handwriting for it in a terminal: `<plugin root>/bin/claudemancy-cli --train <id>`. That needs a mouse in an interactive terminal, so don't run it yourself.

## Common changes

- **New drawing for an existing spell:** copy its file into the user folder under the same name, then replace the drawing. Use `--suggest` to pick a distinct one.
- **Make a spell cast something else:** copy its file into the user folder and change `skill:`, `args:` or `prompt:`.
- **Remove a default:** create `<user folder>/<id>.sigil` containing only `disabled: true` and a `---` line.
- **Accept another way of drawing it:** add another drawing after a further `---` line.

## Format reference

| Key | Meaning |
|---|---|
| `name` | Title in the grimoire and on the cast. Defaults to the file name. |
| `skill` | Skill to invoke. Use this or `prompt`, not both. |
| `args` | Arguments for the skill. |
| `prompt` | An instruction Claude carries out as if the user typed it. |
| `aspect` | Character height ÷ width. Default `2`, right for monospace fonts. |
| `disabled` | `true` hides the spell with this file name from earlier folders. |

Header lines starting with `//` are comments.

Drawings:
- About 20–24 characters wide, single-width lines, proportions as they look in a monospace editor.
- Stroke order and direction don't matter, and one-character gaps are bridged.
- At least 4 ink characters.

Full reference: `docs/sigils.md` in the plugin.
