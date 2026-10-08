# claudemancy

Draw Doctor Strange–style sigils in a terminal canvas to cast Claude Code skills.
Type `/cast`, a glowing Braille canvas pops over your terminal, you draw a sigil,
it resolves into a spinning rune mandala, and Claude invokes the bound skill.

```
/cast                      draw → skill runs
/cast only the auth module draw → skill runs with those extra instructions
/spells                    list every mapped sigil → skill
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
  you finish drawing, then injects "invoke skill X" as context. Esc dismisses
  the spell and blocks the prompt.
- **`bin/claudemancy-popup`** picks the fastest overlay available (see below) and
  reads the result back over a fifo.
- **`bin/claudemancy`** (Go, ~3 MB, no runtime deps) is the TUI: SGR mouse
  tracking (pixel-precise via mode 1016 where supported), Braille 2×4 sub-cell
  rendering in truecolor, $P point-cloud recognition, a procedural mandala per
  spell (derived from a hash of its name).

## Spells

Edit `~/.config/claudemancy/spellbook.json` (created on first `--train`; built-in
default otherwise — see `src/spellbook.default.json`):

```json
{ "name": "Eye of Scrutiny", "shape": "triangle", "skill": "code-review", "args": "high" }
```

Built-in shapes: `circle triangle square star lightning check cross spiral infinity`.
Square vs circle is the weakest pair; train your own to fix that.

**Train your own sigils** — draw it 3–5 times, ⏎ after each:

```sh
bin/claudemancy --train security-review    # binds to a skill of the same name
bin/claudemancy --list
bin/claudemancy --demo star                # watch a cast without drawing
```

Controls: draw with left mouse (multi-stroke is fine) · auto-casts 0.55 s after
you lift · ⏎ cast now · ⌫ / right-click clear · ⇥ toggle grimoire · Esc dismiss ·
any key skips the animation. `max_distance` in the spellbook trades forgiveness for misfires.

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
also flashes the number top-right.

Env knobs: `CLAUDEMANCY_TERMINAL`, `CLAUDEMANCY_SIZE="W H"`, `CLAUDEMANCY_TIMEOUT`.

## Hotkey variant (no `/cast` typing)

Bind `claudemancy-popup` to a key and type the result into the focused Claude
window, e.g. Hyprland (Lua config):

```lua
hl.bind("SUPER + SPACE", hl.dsp.exec_cmd([[sh -c 's=$(~/Projects/claudemancy/bin/claudemancy-popup | jq -r ".skill // empty"); [ -n "$s" ] && wtype "/$s" -k Return']]))
```
