package main

import (
	"hash/fnv"
	"math"
)

// Mandala is the procedural sling-ring circle a recognised spell resolves into.
// Every parameter is derived from the spell name, so each spell has its own circle.
type Mandala struct {
	sides  int     // inner polygon pair (4 = the classic octagram)
	ticks  int     // radial ticks in the inner band
	spin   float64 // base angular velocity, rad/s
	runes  []rune
	offset int
}

func NewMandala(name, glyphs string) *Mandala {
	h := fnv.New64a()
	h.Write([]byte(name))
	s := h.Sum64()
	runes := []rune(glyphs)
	if len(runes) == 0 {
		runes = []rune("*")
	}
	return &Mandala{
		sides:  []int{4, 4, 3, 5, 6, 8}[s%6],
		ticks:  24 + int(s>>8%4)*12,
		spin:   0.9 + float64(s>>16%5)*0.15,
		runes:  runes,
		offset: int(s >> 24 % uint64(len(runes))),
	}
}

// Draw renders the circle at world (cx, cy) with outer radius R. t is seconds
// since the cast began; v scales brightness (>1 runs white-hot).
func (m *Mandala) Draw(c *Canvas, cx, cy, R, t float64, v float32) {
	if R < 1 {
		return
	}
	rot := t * m.spin
	pt := func(r, a float64) (float64, float64) { return cx + r*math.Cos(a), cy + r*math.Sin(a) }

	// Outer double ring with a rune band between.
	c.Circle(cx, cy, R, v)
	c.Circle(cx, cy, R*0.955, v*0.8)
	c.Circle(cx, cy, R*0.80, v*0.9)
	bandR := R * 0.877
	cellH := 4 * c.Aspect
	if n := int(2 * math.Pi * bandR / math.Max(cellH*1.15, 4)); n >= 6 && R*0.155 >= cellH*0.6 {
		for i := 0; i < n; i++ {
			x, y := pt(bandR, rot+2*math.Pi*float64(i)/float64(n))
			flicker := float32(0.85 + 0.15*math.Sin(t*7+float64(i)*1.7))
			c.Glyph(x, y, m.runes[(i*7+m.offset)%len(m.runes)], v*flicker)
		}
	}

	// Counter-rotating tick band.
	for i := 0; i < m.ticks; i++ {
		a := -rot*1.4 + 2*math.Pi*float64(i)/float64(m.ticks)
		inner := 0.70
		if i%3 == 0 {
			inner = 0.64
		}
		x0, y0 := pt(R*0.78, a)
		x1, y1 := pt(R*inner, a)
		c.Line(x0, y0, x1, y1, v*0.75)
	}
	c.Circle(cx, cy, R*0.62, v*0.85)

	// Interlocked polygons, slowly turning the other way.
	for k := 0; k < 2; k++ {
		base := rot*0.6 + float64(k)*math.Pi/float64(m.sides)
		for i := 0; i < m.sides; i++ {
			x0, y0 := pt(R*0.62, base+2*math.Pi*float64(i)/float64(m.sides))
			x1, y1 := pt(R*0.62, base+2*math.Pi*float64(i+1)/float64(m.sides))
			c.Line(x0, y0, x1, y1, v*0.95)
		}
	}

	// Core: small ring, spokes and a sigil.
	c.Circle(cx, cy, R*0.26, v)
	c.Circle(cx, cy, R*0.20, v*0.7)
	for i := 0; i < m.sides*2; i++ {
		a := -rot*2 + math.Pi*float64(i)/float64(m.sides)
		x0, y0 := pt(R*0.26, a)
		x1, y1 := pt(R*0.44, a)
		c.Line(x0, y0, x1, y1, v*0.6)
	}
	c.Glyph(cx, cy, m.runes[m.offset], min(v*1.2, 1.5))
}
