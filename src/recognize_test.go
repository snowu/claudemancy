package main

import (
	"math/rand/v2"
	"testing"
)

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
