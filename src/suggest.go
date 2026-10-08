package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
)

// symbols is a library of candidate sigils for new spells, as strokes in a
// unit box (y down). --suggest tests each against the current spells, so
// symbols that are already taken or easily confused drop out on their own.
var symbols = map[string]func() []Pt{
	"diamond":      func() []Pt { return poly(0, [][2]float64{{0.5, 0}, {1, 0.5}, {0.5, 1}, {0, 0.5}, {0.5, 0}}) },
	"hourglass":    func() []Pt { return poly(0, [][2]float64{{0, 0}, {1, 0}, {0, 1}, {1, 1}, {0, 0}}) },
	"chevron-up":   func() []Pt { return poly(0, [][2]float64{{0, 0.8}, {0.5, 0}, {1, 0.8}}) },
	"chevron-down": func() []Pt { return poly(0, [][2]float64{{0, 0}, {0.5, 0.8}, {1, 0}}) },
	"m":            func() []Pt { return poly(0, [][2]float64{{0, 1}, {0.1, 0}, {0.5, 0.7}, {0.9, 0}, {1, 1}}) },
	"w":            func() []Pt { return poly(0, [][2]float64{{0, 0}, {0.25, 1}, {0.5, 0.35}, {0.75, 1}, {1, 0}}) },
	"plus": func() []Pt {
		return append(poly(0, [][2]float64{{0.5, 0}, {0.5, 1}}), poly(1, [][2]float64{{0, 0.5}, {1, 0.5}})...)
	},
	"arrow-right": func() []Pt {
		return append(poly(0, [][2]float64{{0, 0.5}, {1, 0.5}}), poly(1, [][2]float64{{0.6, 0.1}, {1, 0.5}, {0.6, 0.9}})...)
	},
	"wave": func() []Pt {
		var p []Pt
		for i := 0; i <= 64; i++ {
			t := float64(i) / 64
			p = append(p, Pt{t, 0.5 - 0.3*math.Sin(t*4*math.Pi), 0})
		}
		return p
	},
	"heart": func() []Pt {
		var p []Pt
		for i := 0; i <= 96; i++ {
			a := 2 * math.Pi * float64(i) / 96
			x := 16 * math.Pow(math.Sin(a), 3)
			y := 13*math.Cos(a) - 5*math.Cos(2*a) - 2*math.Cos(3*a) - math.Cos(4*a)
			p = append(p, Pt{0.5 + x/34, 0.5 - y/34, 0})
		}
		return p
	},
	"crescent": func() []Pt {
		outer := arc(0.5, 0.5, 0.5, math.Pi/4, 7*math.Pi/4, 48, 0)
		inner := arc(0.75, 0.5, 0.36, 3*math.Pi/4+0.5, 5*math.Pi/4-0.5, 24, 0)
		for i, j := 0, len(inner)-1; i < j; i, j = i+1, j-1 {
			inner[i], inner[j] = inner[j], inner[i]
		}
		return append(outer, inner...)
	},
	"brackets": func() []Pt { // <>
		return append(poly(0, [][2]float64{{0.45, 0}, {0, 0.5}, {0.45, 1}}), poly(1, [][2]float64{{0.55, 0}, {1, 0.5}, {0.55, 1}})...)
	},
	"bowtie": func() []Pt { return poly(0, [][2]float64{{0, 0}, {1, 1}, {1, 0}, {0, 1}, {0, 0}}) },
	"trident": func() []Pt {
		return append(poly(0, [][2]float64{{0, 0}, {0, 0.4}, {1, 0.4}, {1, 0}}),
			poly(1, [][2]float64{{0.5, 0}, {0.5, 1}})...)
	},
}

func init() {
	// Built-in shapes not used by a default spell are candidates too.
	for _, n := range []string{"spiral", "circle", "triangle", "square", "star", "lightning", "check", "cross", "infinity"} {
		symbols[n] = shapes[n]
	}
}

func symbolNames() []string {
	names := make([]string, 0, len(symbols))
	for n := range symbols {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// wobble simulates a hand-drawn version of a shape: random scale/aspect/rotation,
// per-point jitter, and an over- or under-shot ending.
func wobble(pts []Pt, rng *rand.Rand) []Pt {
	sx := 40 + rng.Float64()*60
	sy := sx * (0.8 + rng.Float64()*0.4)
	rot := (rng.Float64() - 0.5) * 0.35
	cos, sin := math.Cos(rot), math.Sin(rot)
	keep := len(pts) - rng.IntN(len(pts)/12+1)
	out := make([]Pt, 0, keep)
	for _, p := range pts[:keep] {
		x, y := p.X*sx, p.Y*sy
		out = append(out, Pt{x*cos - y*sin + rng.NormFloat64()*1.2, x*sin + y*cos + rng.NormFloat64()*1.2, p.ID})
	}
	return out
}

type Suggestion struct {
	Symbol  string
	Drawing Drawing
	Nearest string  // id of the most similar existing spell
	Dist    float64 // template distance to it
	Hits    int     // simulated hand-drawn casts recognised as the new spell, out of suggestTrials
}

const suggestTrials = 40

// Suggest ranks the symbol library for a new spell: each candidate is drawn as
// ASCII, added to the current spells, and cast suggestTrials times by
// simulated hand; look-alikes of existing spells are left out.
func Suggest(b *Spellbook) []Suggestion {
	var out []Suggestion
	for _, n := range symbolNames() {
		d := Rasterize(symbols[n](), sigilCols, defaultAspect)
		trial := &Spellbook{MaxDistance: b.MaxDistance, Spells: append(append([]Spell(nil), b.Spells...), Spell{ID: "candidate", Drawings: []Drawing{d}})}
		self := len(trial.Spells) - 1

		s := Suggestion{Symbol: n, Drawing: d, Dist: math.Inf(1)}
		for _, p := range closestPairs(trial) {
			if (p.a == self || p.b == self) && p.dist < s.Dist {
				other := p.a + p.b - self
				s.Dist, s.Nearest = p.dist, trial.Spells[other].ID
			}
		}
		if s.Dist < closeCall {
			continue
		}
		r := trial.Recognizer()
		rng := rand.New(rand.NewPCG(11, uint64(len(n))))
		for range suggestTrials {
			if m := r.Recognize(wobble(symbols[n](), rng)); m.Spell == self && m.Confident(b.MaxDistance) {
				s.Hits++
			}
		}
		if s.Hits*10 >= suggestTrials*8 {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].Dist > out[j].Dist
	})
	return out
}

// PrintSuggestions writes the top n suggestions as ready-to-paste drawings.
func PrintSuggestions(b *Spellbook, n int) {
	sug := Suggest(b)
	if len(sug) == 0 {
		fmt.Println("no library symbol is clearly distinct from the current spells; draw your own and run --check")
		return
	}
	fmt.Printf("Most distinct symbols for a new spell (of %d spells loaded), best first.\n", len(b.Spells))
	fmt.Println("Paste a drawing under a `---` line in your .sigil file.")
	for _, s := range sug[:min(n, len(sug))] {
		fmt.Printf("\n%s: recognised %d/%d simulated hand-drawn casts; nearest existing spell %s (%.2f)\n",
			s.Symbol, s.Hits, suggestTrials, s.Nearest, s.Dist)
		for _, l := range s.Drawing.Lines {
			fmt.Println(l)
		}
	}
}
