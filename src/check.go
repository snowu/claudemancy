package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Template distances between two spells' sigils, calibrated by casting wobbled
// hand-drawn shapes at pairs like circle/octagon (0.73: ~30% cast as the other
// spell), circle/pentagon (0.93: ~5%) and square/diamond (1.62: never).
const (
	tooSimilar = 0.8  // hand-drawn versions regularly get cast as each other
	closeCall  = 1.15 // occasional mix-ups and more fizzles
)

type pair struct {
	a, b int
	dist float64
}

// closestPairs returns every pair of spells with the distance between their
// most similar templates, closest first.
func closestPairs(b *Spellbook) []pair {
	r := b.Recognizer()
	best := map[[2]int]float64{}
	for i, ti := range r.templates {
		for _, tj := range r.templates[i+1:] {
			if ti.spell == tj.spell {
				continue
			}
			k := [2]int{min(ti.spell, tj.spell), max(ti.spell, tj.spell)}
			d := greedyCloudMatch(ti.points, tj.points)
			if v, ok := best[k]; !ok || d < v {
				best[k] = d
			}
		}
	}
	var out []pair
	for k, d := range best {
		out = append(out, pair{k[0], k[1], d})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].dist < out[j].dist })
	return out
}

// Check validates the loaded spells plus any extra sigil files (drafts that
// override loaded spells with the same id) and reports look-alike sigils.
// It returns the process exit code: 1 if any file is broken.
func Check(b *Spellbook, files []string) int {
	failed := len(b.Problems) > 0
	for _, p := range b.Problems {
		fmt.Println("✗", p)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Println("✗", err)
			failed = true
			continue
		}
		id := trimExt(f)
		s, err := ParseSigil(id, string(data))
		if err != nil {
			fmt.Printf("✗ %s: %v\n", f, err)
			failed = true
			continue
		}
		s.File, s.Source = f, "draft"
		if i := b.indexOf(id); i >= 0 {
			b.Spells[i] = s
		} else {
			b.Spells = append(b.Spells, s)
		}
	}

	var from []string
	for _, src := range []string{"built-in", "user", "project", "spellbook.json", "draft"} {
		n := 0
		for _, s := range b.Spells {
			if s.Source == src {
				n++
			}
		}
		if n > 0 {
			from = append(from, fmt.Sprintf("%d %s", n, src))
		}
	}
	fmt.Printf("%d spells loaded (%s)\n", len(b.Spells), strings.Join(from, ", "))

	clean := true
	for _, p := range closestPairs(b) {
		a, c := b.Spells[p.a], b.Spells[p.b]
		switch {
		case p.dist < tooSimilar:
			fmt.Printf("✗ %s and %s look too similar (%.2f): hand-drawn versions get cast as each other. Make one more distinct.\n", a.ID, c.ID, p.dist)
			failed, clean = true, false
		case p.dist < closeCall:
			fmt.Printf("⚠ %s and %s are close (%.2f): fine when drawn neatly; add drawings to whichever gets mixed up.\n", a.ID, c.ID, p.dist)
			clean = false
		}
	}
	if clean && !failed {
		fmt.Println("✓ every sigil is distinct")
	}
	if failed {
		return 1
	}
	return 0
}

func trimExt(path string) string {
	base := path
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		base = path[i:]
	}
	if len(base) > len(sigilExt) && base[len(base)-len(sigilExt):] == sigilExt {
		return base[:len(base)-len(sigilExt)]
	}
	return base
}
