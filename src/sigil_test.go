package main

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func TestSigilRoundTrip(t *testing.T) {
	src := `name: Eye of Scrutiny
skill: /code-review
args: high
// comments are allowed in the header
---
   #
  # #
 #####
---
  ##
 #  #
######
`
	s, err := ParseSigil("code-review", src)
	if err != nil {
		t.Fatal(err)
	}
	if s.Skill != "code-review" || s.Args != "high" || len(s.Drawings) != 2 || s.Drawings[0].Aspect != 2 {
		t.Fatalf("parsed %+v", s)
	}
	again, err := ParseSigil("code-review", FormatSigil(s))
	if err != nil || FormatSigil(again) != FormatSigil(s) {
		t.Fatalf("round trip changed the file:\n%s\n---\n%s", FormatSigil(s), FormatSigil(again))
	}
}

func TestSigilErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{"name: x\nskill: a\n", "missing the ---"},
		{"name: x\n---\n####\n", "needs a `skill:` or a `prompt:`"},
		{"skill: a\nprompt: b\n---\n####\n", "pick one"},
		{"skill: a\n---\n", "no drawing"},
		{"skill: a\n---\n##\n", "fewer than 4"},
		{"skill: a\ncolour: red\n---\n####\n", "unknown key"},
		{"prompt: p\nargs: x\n---\n####\n", "only applies to a `skill:`"},
	} {
		if _, err := ParseSigil("x", c.src); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: got %v, want error containing %q", c.src, err, c.want)
		}
	}
	if s, err := ParseSigil("x", "disabled: true\n---\n"); err != nil || !s.Disabled {
		t.Errorf("disabled stub: %v %+v", err, s)
	}
}

// The shipped ASCII sigils must recognise hand-drawn input (simulated by
// wobbling the matching generated shape) and reject random scribbles.
func TestDefaultSpellsRecognise(t *testing.T) {
	spells, problems := LoadSpellDirs([]SpellDir{{"../spells", "built-in"}})
	for _, p := range problems {
		t.Error(p)
	}
	b := &Spellbook{Spells: spells}
	r := b.Recognizer()
	shapeOf := map[string]string{
		"code-review": "triangle", "simplify": "circle", "security-review": "square", "run": "star",
		"init": "lightning", "fewer-permission-prompts": "check", "test-loop": "infinity", "commit": "cross",
	}
	if len(spells) != len(shapeOf) {
		t.Fatalf("loaded %d default spells, test knows %d", len(spells), len(shapeOf))
	}
	rng := rand.New(rand.NewPCG(1, 2))
	const trials = 100
	for i, s := range spells {
		hits := 0
		for range trials {
			if m := r.Recognize(wobble(shapes[shapeOf[s.ID]](), rng)); m.Spell == i && m.Confident(defaultMaxDistance) {
				hits++
			} else if m.Confident(defaultMaxDistance) {
				t.Errorf("a %s was cast as %s", shapeOf[s.ID], spells[m.Spell].ID)
			}
		}
		t.Logf("%-26s %3d/%d confidently correct", s.ID, hits, trials)
		if hits < trials*80/100 {
			t.Errorf("%s recognised only %d/%d", s.ID, hits, trials)
		}
	}
	misfires := 0
	for range 1000 {
		if r.Recognize(scribble(rng)).Confident(defaultMaxDistance) {
			misfires++
		}
	}
	t.Logf("random scribbles accepted: %d/1000", misfires)
	if misfires > 15 {
		t.Errorf("too many scribbles accepted: %d/1000", misfires)
	}
}

func scribble(rng *rand.Rand) []Pt {
	var p []Pt
	x, y := 0.0, 0.0
	for range 40 + rng.IntN(60) {
		x += rng.NormFloat64() * 6
		y += rng.NormFloat64() * 6
		p = append(p, Pt{x, y, 0})
	}
	return p
}

func TestSuggestOffersOnlyFreeDistinctSymbols(t *testing.T) {
	spells, _ := LoadSpellDirs([]SpellDir{{"../spells", "built-in"}})
	b := &Spellbook{MaxDistance: defaultMaxDistance, Spells: spells}
	sug := Suggest(b)
	if len(sug) < 3 {
		t.Fatalf("only %d suggestions", len(sug))
	}
	for _, s := range sug {
		if s.Dist < closeCall || s.Hits*10 < suggestTrials*8 {
			t.Errorf("%s suggested with dist %.2f, %d/%d hits", s.Symbol, s.Dist, s.Hits, suggestTrials)
		}
		// Shapes the defaults already use must never come back.
		switch s.Symbol {
		case "triangle", "circle", "square", "star", "lightning", "check", "cross", "infinity":
			t.Errorf("suggested %s, which a default spell already uses", s.Symbol)
		}
		// A suggestion pasted into a file must parse and pass --check.
		src := FormatSigil(Spell{Name: s.Symbol, Skill: "x", Drawings: []Drawing{s.Drawing}})
		if _, err := ParseSigil(s.Symbol, src); err != nil {
			t.Errorf("%s: %v", s.Symbol, err)
		}
	}
}
