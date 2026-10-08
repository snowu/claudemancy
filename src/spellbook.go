package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed spellbook.default.json
var defaultSpellbook []byte

type Spell struct {
	Name      string         `json:"name"`
	Skill     string         `json:"skill"`
	Args      string         `json:"args,omitempty"`
	Shape     string         `json:"shape,omitempty"`     // built-in sigil from shapes.go
	Templates [][][3]float64 `json:"templates,omitempty"` // trained sigils: [x, y, stroke]
}

type Spellbook struct {
	MaxDistance float64  `json:"max_distance,omitempty"` // higher = more forgiving, more misfires
	Glyphs      string   `json:"glyphs,omitempty"`       // rune alphabet for the mandala bands
	Glow        GlowMode `json:"glow,omitempty"`         // "soft" (default), "full" or "off"
	Spells      []Spell  `json:"spells"`
}

func userSpellbookPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "claudemancy", "spellbook.json")
}

// LoadSpellbook reads path, or the user's spellbook, or the embedded default.
func LoadSpellbook(path string) (*Spellbook, string, error) {
	if path == "" {
		path = userSpellbookPath()
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data, err = defaultSpellbook, nil
	}
	if err != nil {
		return nil, path, err
	}
	var b Spellbook
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, path, err
	}
	if b.MaxDistance == 0 {
		b.MaxDistance = 1.3
	}
	if b.Glyphs == "" {
		b.Glyphs = "ᚠᚢᚦᚨᚱᚲᚷᚹᚺᚾᛁᛃᛇᛈᛉᛊᛏᛒᛖᛗᛚᛜᛞᛟ"
	}
	return &b, path, nil
}

func (b *Spellbook) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (b *Spellbook) Recognizer() *Recognizer {
	r := &Recognizer{}
	for i, s := range b.Spells {
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

// Inscribe stores a trained template under the spell named name, creating the
// spell (bound to a skill of the same name) if it doesn't exist yet.
func (b *Spellbook) Inscribe(name string, pts []Pt) int {
	idx := -1
	for i, s := range b.Spells {
		if s.Name == name || (idx < 0 && s.Skill == name) {
			idx = i
		}
	}
	if idx < 0 {
		b.Spells = append(b.Spells, Spell{Name: name, Skill: name})
		idx = len(b.Spells) - 1
	}
	t := make([][3]float64, len(pts))
	for i, p := range pts {
		t[i] = [3]float64{round2(p.X), round2(p.Y), float64(p.ID)}
	}
	b.Spells[idx].Templates = append(b.Spells[idx].Templates, t)
	return len(b.Spells[idx].Templates)
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
