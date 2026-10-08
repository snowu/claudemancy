package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Spell struct {
	Name   string `json:"name"`
	Skill  string `json:"skill,omitempty"`
	Args   string `json:"args,omitempty"`
	Prompt string `json:"prompt,omitempty"` // cast a freeform instruction instead of a skill

	// Legacy spellbook.json spells (before sigil files).
	Shape     string         `json:"shape,omitempty"`     // built-in sigil from shapes.go
	Templates [][][3]float64 `json:"templates,omitempty"` // trained sigils: [x, y, stroke]

	// From sigil files.
	ID       string    `json:"-"` // filename stem; same ID in a later layer overrides
	Drawings []Drawing `json:"-"`
	Disabled bool      `json:"-"`
	File     string    `json:"-"`
	Source   string    `json:"-"` // built-in, user, project or spellbook.json
}

// Spellbook is the settings from spellbook.json plus every spell: the sigil
// file layers first, then any legacy spells still stored in spellbook.json.
type Spellbook struct {
	MaxDistance float64  `json:"max_distance,omitempty"` // higher = more forgiving, more misfires
	Glyphs      string   `json:"glyphs,omitempty"`       // rune alphabet for the mandala bands
	Glow        GlowMode `json:"glow,omitempty"`         // "soft" (default), "full" or "off"
	Spells      []Spell  `json:"spells,omitempty"`       // legacy; spells now live in .sigil files

	Problems []error `json:"-"` // sigil files that failed to load
}

func userSpellbookPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "claudemancy", "spellbook.json")
}

// LoadSpellbook reads the settings file (default ~/.config/claudemancy/
// spellbook.json, optional) and the spells in dirs. Broken sigil files land in
// Problems rather than failing the load, so one typo can't disable casting.
func LoadSpellbook(settingsPath string, dirs []SpellDir) (*Spellbook, error) {
	if settingsPath == "" {
		settingsPath = userSpellbookPath()
	}
	var b Spellbook
	data, err := os.ReadFile(settingsPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		if err := json.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("%s: %w", settingsPath, err)
		}
	}
	if b.MaxDistance == 0 {
		b.MaxDistance = defaultMaxDistance
	}
	if b.Glyphs == "" {
		b.Glyphs = "ᚠᚢᚦᚨᚱᚲᚷᚹᚺᚾᛁᛃᛇᛈᛉᛊᛏᛒᛖᛗᛚᛜᛞᛟ"
	}

	legacy := b.Spells
	b.Spells, b.Problems = LoadSpellDirs(dirs)
	for _, s := range legacy {
		// Pre-sigil spellbooks copied the built-in shape spells; the sigil
		// files replace those, so only spells with trained drawings carry over.
		if len(s.Templates) == 0 {
			continue
		}
		s.ID, s.Source, s.File = slug(s.Name), "spellbook.json", settingsPath
		b.Spells = append(b.Spells, s)
	}
	return &b, nil
}

func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// validID reports whether id can be a sigil filename stem.
func validID(id string) bool { return id != "" && slug(id) == id }

func (b *Spellbook) Recognizer() *Recognizer {
	r := &Recognizer{}
	for i, s := range b.Spells {
		for _, d := range s.Drawings {
			r.AddTilted(i, d.Strokes())
		}
		if gen, ok := shapes[s.Shape]; ok {
			r.AddTilted(i, gen())
		}
		for _, t := range s.Templates {
			r.Add(i, templatePts(t))
		}
	}
	return r
}

// GlowMode is how much ember background glow strokes get. Cell backgrounds are
// whole character cells, so wider glow reads as blocks.
type GlowMode int

const (
	GlowSoft GlowMode = iota // shimmering dot halo beside lines, no cell backgrounds
	GlowFull                 // dot halo plus ember cell backgrounds (blocky)
	GlowOff
)

// UnmarshalJSON accepts "soft"/"full"/"off", and true/false from 0.2.1 configs.
func (g *GlowMode) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"soft"`, `""`, "null":
		*g = GlowSoft
	case `"full"`, "true":
		*g = GlowFull
	case `"off"`, "false":
		*g = GlowOff
	default:
		return fmt.Errorf(`glow must be "soft", "full" or "off", got %s`, b)
	}
	return nil
}

func (g GlowMode) MarshalJSON() ([]byte, error) {
	switch g {
	case GlowSoft:
		return []byte(`"soft"`), nil
	case GlowFull:
		return []byte(`"full"`), nil
	case GlowOff:
		return []byte(`"off"`), nil
	}
	return nil, fmt.Errorf("invalid glow mode %d", int(g))
}

// templatePts decodes a stored [x, y, stroke] template.
func templatePts(t [][3]float64) []Pt {
	pts := make([]Pt, len(t))
	for i, p := range t {
		pts[i] = Pt{p[0], p[1], int(p[2])}
	}
	return pts
}

// Inscribe adds a drawing of pts to the user's sigil file for id. A new file
// starts from the spell that currently has that id, so overriding a built-in
// keeps its drawings, or else from a new spell for the skill named id.
// It returns the file written and how many drawings the spell now has.
func (b *Spellbook) Inscribe(id string, pts []Pt) (string, int, error) {
	path := filepath.Join(userSpellsDir(), id+sigilExt)
	var s Spell
	if data, err := os.ReadFile(path); err == nil {
		if s, err = ParseSigil(id, string(data)); err != nil {
			return path, 0, err
		}
	} else if i := b.indexOf(id); i >= 0 && b.Spells[i].Source != "spellbook.json" {
		s = b.Spells[i]
	} else {
		s = Spell{Name: titleize(id), Skill: id}
	}
	s.Drawings = append(s.Drawings, Rasterize(pts, sigilCols, defaultAspect))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, 0, err
	}
	return path, len(s.Drawings), os.WriteFile(path, []byte(FormatSigil(s)), 0o644)
}

func (b *Spellbook) indexOf(id string) int {
	for i, s := range b.Spells {
		if s.ID == id {
			return i
		}
	}
	return -1
}

// Migrate turns legacy trained spells in spellbook.json into user sigil files
// and drops them from the settings file. It returns the files written.
func (b *Spellbook) Migrate(settingsPath string) ([]string, error) {
	var written []string
	for _, s := range b.Spells {
		if s.Source != "spellbook.json" {
			continue
		}
		path := filepath.Join(userSpellsDir(), s.ID+sigilExt)
		if _, err := os.Stat(path); err == nil {
			return written, fmt.Errorf("%s already exists; move it aside and migrate again", path)
		}
		out := Spell{Name: s.Name, Skill: s.Skill, Args: s.Args, Prompt: s.Prompt}
		for _, t := range s.Templates {
			out.Drawings = append(out.Drawings, Rasterize(templatePts(t), sigilCols, defaultAspect))
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(path, []byte(FormatSigil(out)), 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	// Drop only the "spells" key, so settings left at their defaults stay
	// unset and keep following future defaults.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return written, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return written, err
	}
	delete(raw, "spells")
	data, err = json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return written, err
	}
	return written, os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}
