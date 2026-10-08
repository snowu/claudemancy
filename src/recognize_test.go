package main

import (
	"math"
	"math/rand/v2"
	"testing"
)

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

func TestBuiltinShapesAreDistinguishable(t *testing.T) {
	r := &Recognizer{}
	names := shapeNames()
	for i, n := range names {
		r.AddTilted(i, shapes[n]())
	}
	rng := rand.New(rand.NewPCG(1, 2))
	const trials = 60
	for i, n := range names {
		hits := 0
		for range trials {
			if m := r.Recognize(wobble(shapes[n](), rng)); m.Spell == i && m.Confident(defaultMaxDistance) {
				hits++
			}
		}
		t.Logf("%-10s %2d/%d confidently correct", n, hits, trials)
		if hits < trials*80/100 { // square vs circle is the weakest pair (~82%)
			t.Errorf("%s recognised only %d/%d", n, hits, trials)
		}
	}
}

func TestScribblesAreRejected(t *testing.T) {
	r := &Recognizer{}
	names := shapeNames()
	for i, n := range names {
		r.AddTilted(i, shapes[n]())
	}
	rng := rand.New(rand.NewPCG(3, 4))
	misfires := 0
	for range 400 {
		var p []Pt
		x, y := 0.0, 0.0
		for range 40 + rng.IntN(60) {
			x += rng.NormFloat64() * 6
			y += rng.NormFloat64() * 6
			p = append(p, Pt{x, y, 0})
		}
		if r.Recognize(p).Confident(defaultMaxDistance) {
			misfires++
		}
	}
	t.Logf("random scribbles accepted: %d/400", misfires)
	if misfires > 8 {
		t.Errorf("too many scribbles accepted: %d/400", misfires)
	}
}
