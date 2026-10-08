package main

import (
	"math"
	"sort"
)

// Built-in sigils, generated rather than trained so spells work out of the box.
// Coordinates are y-down, roughly in a unit box.
var shapes = map[string]func() []Pt{
	"circle":   func() []Pt { return arc(0.5, 0.5, 0.5, 0, 2*math.Pi, 64, 0) },
	"triangle": func() []Pt { return poly(0, [][2]float64{{0.5, 0}, {1, 0.87}, {0, 0.87}, {0.5, 0}}) },
	"square":   func() []Pt { return poly(0, [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}) },
	"star": func() []Pt {
		var v [][2]float64
		for i := 0; i <= 5; i++ {
			a := -math.Pi/2 + float64(i*2%5)*2*math.Pi/5
			v = append(v, [2]float64{0.5 + 0.5*math.Cos(a), 0.5 + 0.5*math.Sin(a)})
		}
		return poly(0, v)
	},
	"spiral": func() []Pt {
		var p []Pt
		for i := 0; i <= 120; i++ {
			t := float64(i) / 120
			a := t * 3 * 2 * math.Pi
			p = append(p, Pt{0.5 + 0.5*t*math.Cos(a), 0.5 + 0.5*t*math.Sin(a), 0})
		}
		return p
	},
	"lightning": func() []Pt {
		return poly(0, [][2]float64{{0.65, 0}, {0.2, 0.55}, {0.7, 0.45}, {0.3, 1}})
	},
	"infinity": func() []Pt {
		var p []Pt
		for i := 0; i <= 96; i++ {
			a := 2 * math.Pi * float64(i) / 96
			d := 1 + math.Sin(a)*math.Sin(a)
			p = append(p, Pt{0.5 + 0.5*math.Cos(a)/d, 0.5 + 0.5*math.Sin(a)*math.Cos(a)/d, 0})
		}
		return p
	},
	"check": func() []Pt { return poly(0, [][2]float64{{0, 0.55}, {0.35, 1}, {1, 0}}) },
	"cross": func() []Pt {
		return append(poly(0, [][2]float64{{0, 0}, {1, 1}}), poly(1, [][2]float64{{1, 0}, {0, 1}})...)
	},
}

func shapeNames() []string {
	var names []string
	for k := range shapes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func arc(cx, cy, r, a0, a1 float64, n, id int) []Pt {
	var p []Pt
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(n)
		p = append(p, Pt{cx + r*math.Cos(a), cy + r*math.Sin(a), id})
	}
	return p
}

// poly densifies a polyline so its points are evenly spread along the path.
func poly(id int, v [][2]float64) []Pt {
	var p []Pt
	for i := 1; i < len(v); i++ {
		for s := 0; s < 16; s++ {
			t := float64(s) / 16
			p = append(p, Pt{v[i-1][0] + (v[i][0]-v[i-1][0])*t, v[i-1][1] + (v[i][1]-v[i-1][1])*t, id})
		}
	}
	return append(p, Pt{v[len(v)-1][0], v[len(v)-1][1], id})
}
