# claudemancy

Draw Doctor Strange–style sigils in a terminal canvas to cast Claude Code skills.
Type `/cast`, a glowing Braille canvas pops over your terminal, you draw a sigil,
it resolves into a spinning rune mandala, and Claude invokes the bound skill.

```
/cast                      draw → skill runs
/cast only the auth module draw → skill runs with those extra instructions
/spells                    list every spell, its sigil and its file
```

The canvas also shows a **grimoire** panel listing each spell with its sigil;
Tab toggles it.

## Install

Needs Go ≥1.24 (the canvas binary builds itself on first `/cast`), `jq`, and a
mouse-capable terminal.

Inside Claude Code:

```
/plugin marketplace add snowu/claudemancy
/plugin install claudemancy@claudemancy
```

Or from a clone, for one session: `claude --plugin-dir path/to/claudemancy`
(run `./build.sh` first to skip the first-use build).

## How it works

- **`UserPromptSubmit` hook** (`bin/cast-hook`) catches `/cast` *before* the model
  runs, so the canvas pops without waiting on an LLM round-trip. It blocks until
  you finish drawing, then injects "invoke skill X" (or a prompt spell's
  instruction) as context. Esc dismisses the spell and blocks the prompt.
- **`bin/claudemancy-popup`** picks the fastest overlay available (see below) and
  reads the result back over a fifo.
- **`bin/claudemancy`** (Go, ~3 MB, no runtime deps) is the TUI: SGR mouse
  tracking (pixel-precise via mode 1016 where supported), Braille 2×4 sub-cell
  rendering in truecolor, $P point-cloud recognition against the traced
  `.sigil` drawings, a procedural mandala per spell (derived from a hash of its
  name).

## Spells

Every sigil is drawn in **one stroke**, without lifting the mouse, and is
distinct enough from the others that a quick, sloppy version still casts the
right spell. Every spell is a plain-text `.sigil` file: a short header, then an ASCII drawing
of the sigil. Humans and agents can add a spell by adding a file.

```
name: Seal of the Vishanti
prompt: Commit the current changes, push, and open a pull request.
---
##                  ##
 ####             ###
    ###         ###
      ###     ###
        #######
           #
```

| Draw | Spell | Casts |
|---|---|---|
| triangle | Eye of Scrutiny | `/code-review` |
| circle | Circle of Purity | `/simplify` |
| M | Ward of Cyttorak | `/security-review` |
| lightning bolt | Bolt of Genesis | `/init` |
| check mark | Seal of Passage | `/fewer-permission-prompts` |
| infinity | Ouroboros Loop | prompt: run the tests, fix failures, repeat |
| V | Seal of the Vishanti | prompt: commit, push and open a pull request |

Spells load from three folders. A later folder overrides an earlier one by
file name:

1. **built-in**: [`spells/`](spells) in this repo
2. **user**: `~/.config/claudemancy/spells/`
3. **project**: `<repo>/.claude/claudemancy/spells/`, shared with your team

To change a default, copy its file into your user or project folder and edit
it. To remove one, add a same-named file containing `disabled: true` and `---`.
A spell can cast a `skill:` (with optional `args:`) or a freeform `prompt:`.

Full format and drawing tips: [docs/sigils.md](docs/sigils.md).

**Or let Claude do it:** the plugin ships an **inscribe** skill. Say "add a spell
that runs /deploy" (or run `/claudemancy:inscribe deploy`). Claude picks a free,
easy-to-recognise symbol, writes the file and validates it.

```sh
bin/claudemancy-cli --list               # every spell, its source file, the folders
bin/claudemancy-cli --suggest 5          # most distinct free symbols, drawn and ready to paste
bin/claudemancy-cli --check [draft.sigil] # validate files, flag look-alike sigils
bin/claudemancy-cli --train deploy       # draw it, ⏎ to add each drawing to your user file
bin/claudemancy-cli --migrate            # move pre-0.3 trained spells out of spellbook.json
bin/claudemancy --demo chevron-down      # watch a cast without drawing
```

`~/.config/claudemancy/spellbook.json` is now just settings: `glow`,
`glyphs` and `max_distance`.

Glow: `"glow": "soft"` (default) scatters a shimmering halo of ember dots beside
every line, with no cell backgrounds, so nothing renders as boxes; `"full"` adds
ember cell backgrounds (warmer, but shows character-cell blocks); `"off"` is
plain lines.

Controls: draw with left mouse (multi-stroke is fine) · auto-casts 0.55 s after
you lift · ⏎ cast now · ⌫ / right-click clear · ⇥ toggle grimoire · Esc dismiss ·
any key skips the animation. `max_distance` in spellbook.json trades forgiveness
for misfires (default 1.22).

## Portability

| Layer | Works on | Notes |
|---|---|---|
| Canvas TUI | any terminal with truecolor + SGR mouse: Ghostty, kitty, WezTerm, foot, Alacritty, iTerm2, Windows Terminal | Pixel-precise strokes need mode 1016 (Ghostty, kitty, WezTerm, foot); others fall back to cell precision, still fine. Runes need a font with Runic (e.g. Noto Sans Runic) or set `"glyphs"` in the spellbook. |
| Binary | Linux, macOS, BSD | `GOOS=darwin ./build.sh` etc. Not native Windows (use WSL). |
| Popup | **tmux anywhere** (`display-popup`, overlays the pane itself) · Hyprland (rule auto-registered) · any other WM via a window rule | Matches on title `claudemancy`. |

Window rules for other tiling WMs:

```
# sway / i3
for_window [title="^claudemancy$"] floating enable, resize set 60 ppt 70 ppt, move position center, sticky enable
# niri
window-rule { match title="^claudemancy$"; open-floating true; }
```

GNOME/KDE float new windows already. macOS: run Claude Code inside tmux for the
popup (or `CLAUDEMANCY_TERMINAL=kitty`).

Popup latency on Ghostty + Hyprland: ~165 ms launch → first frame (it reuses the
running Ghostty over D-Bus; a cold Ghostty start is ~900 ms). Measure yours:
`CLAUDEMANCY_LATENCY_LOG=/tmp/lat bin/claudemancy-popup --demo circle` — the canvas
also flashes the number bottom-right.

Env knobs: `CLAUDEMANCY_TERMINAL`, `CLAUDEMANCY_SIZE="W H"`, `CLAUDEMANCY_TIMEOUT`.

## Hotkey variant (no `/cast` typing)

Bind `claudemancy-popup` to a key and type the result into the focused Claude
window, e.g. Hyprland (Lua config):

```lua
hl.bind("SUPER + SPACE", hl.dsp.exec_cmd([[sh -c 's=$(~/Projects/claudemancy/bin/claudemancy-popup | jq -r ".skill // empty"); [ -n "$s" ] && wtype "/$s" -k Return']]))
```
