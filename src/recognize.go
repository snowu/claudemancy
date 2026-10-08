package main

import "math"

// $P point-cloud recognizer (Vatavu, Anthony, Wobbrock 2012): stroke order,
// direction and count independent, so sigils can be drawn however feels natural.

type Pt struct {
	X, Y float64
	ID   int // stroke index
}

const cloudN = 32

type template struct {
	spell  int
	points []Pt
}

type Recognizer struct{ templates []template }

func (r *Recognizer) Add(spell int, pts []Pt) {
	if len(pts) >= 2 {
		r.templates = append(r.templates, template{spell, normalizeCloud(pts)})
	}
}

// AddTilted adds the template plus slightly rotated copies; $P is not rotation
// invariant and hand-drawn squares otherwise drift toward circles.
func (r *Recognizer) AddTilted(spell int, pts []Pt) {
	for _, deg := range []float64{-12, 0, 12} {
		a := deg * math.Pi / 180
		cos, sin := math.Cos(a), math.Sin(a)
		rot := make([]Pt, len(pts))
		for i, p := range pts {
			rot[i] = Pt{p.X*cos - p.Y*sin, p.X*sin + p.Y*cos, p.ID}
		}
		r.Add(spell, rot)
	}
}

type Match struct {
	Spell int
	Dist  float64 // $P cloud distance to the best template
	Ratio float64 // Dist / distance to the best *other* spell (0 if only one spell)
}

// Confident rejects scribbles: measured on synthetic hand-drawn sigils vs random
// walks, dist < 1.3 && ratio < 0.85 accepts ~96% of sigils and ~1% of scribbles.
func (m Match) Confident(maxDist float64) bool {
	return m.Spell >= 0 && m.Dist < maxDist && m.Ratio < 0.85
}

// Score is a 0..1 display value (1 = perfect match).
func (m Match) Score() float64 { return math.Min(math.Max(1.6-m.Dist, 0), 1) }

func (r *Recognizer) Recognize(pts []Pt) Match {
	if len(r.templates) == 0 || len(pts) < 2 {
		return Match{Spell: -1}
	}
	cand := normalizeCloud(pts)
	perSpell := map[int]float64{}
	for _, t := range r.templates {
		d := greedyCloudMatch(cand, t.points)
		if v, ok := perSpell[t.spell]; !ok || d < v {
			perSpell[t.spell] = d
		}
	}
	m := Match{Spell: -1, Dist: math.Inf(1)}
	second := math.Inf(1)
	for spell, d := range perSpell {
		if d < m.Dist {
			second, m.Dist, m.Spell = m.Dist, d, spell
		} else if d < second {
			second = d
		}
	}
	if !math.IsInf(second, 1) {
		m.Ratio = m.Dist / second
	}
	return m
}

func normalizeCloud(pts []Pt) []Pt {
	p := resample(pts, cloudN)
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, q := range p {
		minX, minY = math.Min(minX, q.X), math.Min(minY, q.Y)
		maxX, maxY = math.Max(maxX, q.X), math.Max(maxY, q.Y)
	}
	size := math.Max(math.Max(maxX-minX, maxY-minY), 1e-9)
	var cx, cy float64
	for i := range p {
		p[i].X = (p[i].X - minX) / size
		p[i].Y = (p[i].Y - minY) / size
		cx += p[i].X
		cy += p[i].Y
	}
	cx, cy = cx/float64(len(p)), cy/float64(len(p))
	for i := range p {
		p[i].X -= cx
		p[i].Y -= cy
	}
	return p
}

func dist(a, b Pt) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func resample(in []Pt, n int) []Pt {
	pts := append([]Pt(nil), in...)
	var length float64
	for i := 1; i < len(pts); i++ {
		if pts[i].ID == pts[i-1].ID {
			length += dist(pts[i-1], pts[i])
		}
	}
	interval := length / float64(n-1)
	out := []Pt{pts[0]}
	if interval <= 0 {
		for len(out) < n {
			out = append(out, pts[0])
		}
		return out
	}
	var acc float64
	for i := 1; i < len(pts) && len(out) < n; i++ {
		if pts[i].ID != pts[i-1].ID {
			continue
		}
		d := dist(pts[i-1], pts[i])
		if acc+d >= interval && d > 0 {
			t := (interval - acc) / d
			q := Pt{pts[i-1].X + t*(pts[i].X-pts[i-1].X), pts[i-1].Y + t*(pts[i].Y-pts[i-1].Y), pts[i].ID}
			out = append(out, q)
			pts = append(pts[:i], append([]Pt{q}, pts[i:]...)...)
			acc = 0
		} else {
			acc += d
		}
	}
	for len(out) < n {
		out = append(out, pts[len(pts)-1])
	}
	return out
}

func greedyCloudMatch(a, b []Pt) float64 {
	step := int(math.Floor(math.Pow(float64(len(a)), 0.5)))
	best := math.Inf(1)
	for i := 0; i < len(a); i += step {
		best = math.Min(best, math.Min(cloudDistance(a, b, i), cloudDistance(b, a, i)))
	}
	return best
}

func cloudDistance(a, b []Pt, start int) float64 {
	n := len(a)
	matched := make([]bool, n)
	var sum float64
	i := start
	for {
		idx, min := -1, math.Inf(1)
		for j := range matched {
			if !matched[j] {
				if d := dist(a[i], b[j]); d < min {
					min, idx = d, j
				}
			}
		}
		matched[idx] = true
		weight := 1 - float64((i-start+n)%n)/float64(n)
		sum += weight * min
		i = (i + 1) % n
		if i == start {
			return sum
		}
	}
}
