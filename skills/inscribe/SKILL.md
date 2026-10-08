---
name: inscribe
description: Add, change, override or disable claudemancy spells, the sigil drawings that cast a skill or a prompt when the user draws them after /cast. Use when the user wants a new spell, a different drawing for a skill, a spell that runs an instruction, or to customise or remove a default spell.
---

# Inscribe a claudemancy spell

A spell is one plain-text `.sigil` file: a short header, a `---` line, then an ASCII drawing of the sigil. Any character other than a space or `.` is ink.

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

## Header keys

| Key | Meaning |
|---|---|
| `name` | Title shown in the grimoire and on the cast. Defaults to the file name. |
| `skill` | Skill to invoke, without the slash, e.g. `code-review` or `my-plugin:deploy`. |
| `args` | Optional arguments passed to that skill. |
| `prompt` | Instead of `skill`: an instruction Claude carries out as if the user typed it. |
| `aspect` | Height ÷ width of a character cell. Default `2`, right for monospace fonts. |
| `disabled` | `true` hides a spell with the same file name from an earlier folder. Needs no drawing. |

Use exactly one of `skill` or `prompt`. Lines starting with `//` in the header are comments.

## Where the file goes

Spells load from three folders. A file in a later folder replaces the earlier one with the **same file name**:

1. Built-in: `spells/` in the plugin. Don't edit these in an installed plugin; updates overwrite them.
2. User: `~/.config/claudemancy/spells/`. Personal spells; use this by default.
3. Project: `<repo>/.claude/claudemancy/spells/`. Shared with everyone working in that repo.

- **To change a default's drawing or what it casts:** copy the built-in file into the user (or project) folder under the same name and edit the copy.
- **To remove a default:** create a file with that name containing only `disabled: true` and a `---` line.
- **New spells** use a new lowercase file name with dashes, e.g. `deploy-staging.sigil`.

## Drawing well

- Draw about 20–24 characters wide with single-width lines. Keep the shape's proportions as they look in a monospace editor.
- The recognizer ignores stroke order and direction, so only the shape matters, not how it's traced.
- Pick a shape that is clearly different from the existing spells. Run the list command below to see them.
- To accept more than one way of drawing it, add another drawing after another `---` line.

## Check your work

The plugin root is two directories above this SKILL.md. Run:

```sh
<plugin root>/bin/claudemancy-cli --list --project "$PWD"                 # existing spells and folders
<plugin root>/bin/claudemancy-cli --check --project "$PWD" [draft.sigil]  # validate
```

`--check` parses every spell plus any draft files you pass, and compares each pair of sigils:

- `✗ … look too similar` (exit 1): hand-drawn versions get cast as each other. Redraw one of them to be more distinct.
- `⚠ … are close`: usually fine. Mention it to the user.

Fix every `✗` before finishing, then tell the user the file path and that they can try it with `/cast`.

The user can also train a sigil by drawing it: `<plugin root>/bin/claudemancy-cli --train <file-name>` opens the canvas and appends each drawing (⏎ to save) to their user sigil file. That needs an interactive terminal, so suggest it rather than running it yourself.
