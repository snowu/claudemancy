# Sigil files

A spell is one `.sigil` file. The file name, without `.sigil`, is the spell's
**id**: it decides which file overrides which.

```
name: Eye of Scrutiny
skill: code-review
// optional comment lines start with //
---
           #
          # #
         #   #
        #     #
       #       #
      #         #
     #           #
    #             #
   #               #
  #                 #
 #                   #
#######################
---
          #
         # #
        #   #
       #     #
      #########
```

The header ends at the first line that is exactly `---`. Everything after it is
the drawing. Further `---` lines start extra drawings of the same spell:
alternative ways to draw it that the recognizer also accepts.

## Header

| Key | Required | Meaning |
|---|---|---|
| `name` | no | Title in the grimoire and on the cast. Defaults to the id. |
| `skill` | one of these two | Skill to invoke, slash optional: `code-review`, `my-plugin:deploy`. |
| `prompt` | one of these two | An instruction Claude carries out as if you had typed it. |
| `args` | no | Arguments for the `skill`. |
| `aspect` | no | Character cell height ÷ width, default `2`. Only change it if your drawings look stretched. |
| `disabled` | no | `true` hides any spell with this id from earlier folders. Needs no drawing. |

Unknown keys are errors, so typos don't fail silently.

## Drawings

- Any character other than a space or `.` is ink. `#` is the convention.
- Draw about 20–24 characters wide with single-width lines, so it looks right
  in a monospace editor. Size and position don't matter, since drawings are
  normalised.
- Order and direction don't matter either. The recognizer compares shapes as
  point clouds, so a square drawn clockwise or anticlockwise, in one stroke or
  four, matches the same file.
- Small gaps are fine: the tracer bridges one-character gaps like `# # #`.
- Every drawing needs at least 4 ink characters.

## Folders and overrides

Spells load in this order. A file in a later folder replaces the spell with
the same id from an earlier one:

| Layer | Folder | Use it for |
|---|---|---|
| built-in | `spells/` in the plugin | The defaults. Edited only in this repo; updates replace an installed copy. |
| user | `~/.config/claudemancy/spells/` (or `$XDG_CONFIG_HOME/claudemancy/spells/`) | Your personal spells and overrides |
| project | `<repo>/.claude/claudemancy/spells/` | Spells for everyone working in that repo; commit them |

- **Change a default:** copy `spells/code-review.sigil` to your user folder
  and edit the drawing, the name, or what it casts.
- **Remove a default:** create `~/.config/claudemancy/spells/commit.sigil`
  containing `disabled: true` and then `---`.
- **Add a spell:** create a new file, e.g. `deploy-staging.sigil`. Ids use
  lowercase letters, digits and dashes.

A broken file is skipped and reported by `/spells` and `--check`. It never
stops the other spells from casting.

## Tools

`bin/claudemancy-cli` builds the binary if needed and passes its flags through.

| Command | What it does |
|---|---|
| `--list --project .` | Every loaded spell with a Braille preview, what it casts, and its file. Also lists the folders. Same as `/spells`. |
| `--suggest N --project .` | Tests a library of ready-drawn symbols (chevrons, arrow, plus, diamond, M, W, bowtie, crescent, …) against the current spells. Prints the N most distinct that also recognise in at least 80% of simulated hand-drawn casts, as drawings to paste. |
| `--check --project . [draft.sigil …]` | Validates every file, plus drafts (which override loaded spells with the same id). Then compares each pair of sigils. Exits 1 on errors. |
| `--train <id>` | Opens the canvas. Each drawing you save with ⏎ is appended to `~/.config/claudemancy/spells/<id>.sigil`. If no such file exists yet, it starts from the current spell with that id, or creates a spell for the skill named `<id>`. |
| `--migrate` | Converts spells trained before 0.3 (stored as point arrays in `spellbook.json`) into user sigil files. |

### Look-alike sigils

`--check` measures how far apart every pair of sigils is. The thresholds were
calibrated by casting synthetic hand-drawn shapes:

| Distance | Result | Example pair |
|---|---|---|
| < 0.8 | `✗` too similar: hand-drawn versions regularly get cast as each other | circle vs octagon (0.73: ~30% cast as the other) |
| 0.8 – 1.15 | `⚠` close: occasional mix-ups or fizzles | circle vs square (0.98), circle vs pentagon (0.93: ~5%) |
| ≥ 1.15 | fine | square vs diamond (1.62: never) |

Fix a `✗` by making one sigil more distinct. For a `⚠`, a second drawing of
whichever spell gets mixed up usually helps.

## Recognition quality

`src/sigil_test.go` casts 100 simulated hand-drawn versions of each default and
1000 random scribbles against the shipped files. Every default lands
95–100% confident and correct, no attempt is cast as the wrong spell, and
about 0.5% of scribbles are accepted. Run it after changing a default:

```sh
go test ./src -run DefaultSpells -v
```
