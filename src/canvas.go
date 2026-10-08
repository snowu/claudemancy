package main

import (
	"bytes"
	"math"
	"strconv"
)

type RGB struct{ R, G, B uint8 }

// Canvas is a Braille dot field (2x4 dots per cell) plus a per-cell glyph
// overlay. Geometry is drawn in "world" units: x in dots, y in dots scaled by
// the dot aspect ratio so circles stay round regardless of font metrics.
type Canvas struct {
	Cols, Rows int
	DW, DH     int
	Aspect     float64 // dot height / dot width in pixels
	I          []float32
	L          []float32 // line-only intensity (no sparks); drives the glow
	glyph      []rune
	glyphV     []float32
	glyphC     []RGB
	glyphFixed []bool
	energy     []float32
	Fizzle     float32 // 0..1 drains the palette to ash
	Glow       GlowMode
	buf        bytes.Buffer
}

func NewCanvas(cols, rows int, aspect float64) *Canvas {
	n := cols * rows
	return &Canvas{
		Cols: cols, Rows: rows, DW: cols * 2, DH: rows * 4, Aspect: aspect,
		I:     make([]float32, cols*2*rows*4),
		L:     make([]float32, cols*2*rows*4),
		glyph: make([]rune, n), glyphV: make([]float32, n), glyphC: make([]RGB, n), glyphFixed: make([]bool, n),
		energy: make([]float32, n),
	}
}

// WorldW and WorldH are the canvas extents in world units.
func (c *Canvas) WorldW() float64 { return float64(c.DW) }
func (c *Canvas) WorldH() float64 { return float64(c.DH) * c.Aspect }

func (c *Canvas) Clear() {
	clear(c.I)
	clear(c.L)
	clear(c.glyph)
}

// Plot lights a dot that belongs to a line or ring, so it also feeds the glow.
func (c *Canvas) Plot(wx, wy float64, v float32) {
	if i, ok := c.dot(wx, wy); ok {
		c.I[i], c.L[i] = max(c.I[i], v), max(c.L[i], v)
	}
}

// Spark lights a free-floating dot that never glows its cell.
func (c *Canvas) Spark(wx, wy float64, v float32) {
	if i, ok := c.dot(wx, wy); ok {
		c.I[i] = max(c.I[i], v)
	}
}

func (c *Canvas) dot(wx, wy float64) (int, bool) {
	x, y := int(math.Floor(wx)), int(math.Floor(wy/c.Aspect))
	if x < 0 || y < 0 || x >= c.DW || y >= c.DH {
		return 0, false
	}
	return y*c.DW + x, true
}

func (c *Canvas) Line(x0, y0, x1, y1 float64, v float32) { c.line(x0, y0, x1, y1, v, c.Plot) }

// FaintLine draws without glow, for small UI art like the grimoire sigils.
func (c *Canvas) FaintLine(x0, y0, x1, y1 float64, v float32) { c.line(x0, y0, x1, y1, v, c.Spark) }

func (c *Canvas) line(x0, y0, x1, y1 float64, v float32, plot func(x, y float64, v float32)) {
	n := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0)/c.Aspect)) + 1
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		plot(x0+(x1-x0)*t, y0+(y1-y0)*t, v)
	}
}

func (c *Canvas) Circle(cx, cy, r float64, v float32) {
	if r <= 0 {
		return
	}
	n := int(2*math.Pi*r*1.6) + 8
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		c.Plot(cx+r*math.Cos(a), cy+r*math.Sin(a), v)
	}
}

func (c *Canvas) cellAt(wx, wy float64) (int, bool) {
	x, y := int(math.Floor(wx/2)), int(math.Floor(wy/c.Aspect/4))
	if x < 0 || y < 0 || x >= c.Cols || y >= c.Rows {
		return 0, false
	}
	return y*c.Cols + x, true
}

// Glyph places a character whose colour follows the heat palette.
func (c *Canvas) Glyph(wx, wy float64, ch rune, v float32) {
	if i, ok := c.cellAt(wx, wy); ok {
		c.glyph[i], c.glyphV[i], c.glyphFixed[i] = ch, v, false
	}
}

// Text writes a fixed-colour string starting at a cell.
func (c *Canvas) Text(col, row int, s string, col3 RGB) {
	if row < 0 || row >= c.Rows {
		return
	}
	for _, ch := range s {
		if col >= 0 && col < c.Cols {
			i := row*c.Cols + col
			c.glyph[i], c.glyphC[i], c.glyphFixed[i] = ch, col3, true
		}
		col++
	}
}

var heatStops = []struct {
	v       float32
	r, g, b float32
}{
	{0.0, 70, 12, 0},
	{0.35, 190, 55, 0},
	{0.7, 255, 130, 20},
	{1.0, 255, 190, 80},
	{1.5, 255, 248, 225},
}

func (c *Canvas) heat(v float32) RGB {
	s := heatStops
	r, g, b := s[len(s)-1].r, s[len(s)-1].g, s[len(s)-1].b
	for i := 1; i < len(s); i++ {
		if v <= s[i].v {
			t := (v - s[i-1].v) / (s[i].v - s[i-1].v)
			if t < 0 {
				t = 0
			}
			r = s[i-1].r + (s[i].r-s[i-1].r)*t
			g = s[i-1].g + (s[i].g-s[i-1].g)*t
			b = s[i-1].b + (s[i].b-s[i-1].b)*t
			break
		}
	}
	if f := c.Fizzle; f > 0 {
		grey := (r + g + b) / 3 * 0.55
		r, g, b = r+(grey-r)*f, g+(grey-g)*f, b+(grey+10-b)*f
	}
	return RGB{uint8(r), uint8(g), uint8(b)}
}

var brailleBit = [4][2]rune{{0x01, 0x08}, {0x02, 0x10}, {0x04, 0x20}, {0x40, 0x80}}

// cellBits returns the Braille dot pattern of a cell and its brightest dot.
func (c *Canvas) cellBits(cx, cy int) (rune, float32) {
	var bits rune
	var peak float32
	for dy := 0; dy < 4; dy++ {
		row := (cy*4 + dy) * c.DW
		for dx := 0; dx < 2; dx++ {
			v := c.I[row+cx*2+dx]
			if v > 0.08 {
				bits |= brailleBit[dy][dx]
			}
			peak = max(peak, v)
		}
	}
	return bits, peak
}

// Render serialises the frame into one synchronized-output write.
func (c *Canvas) Render() []byte {
	w := &c.buf
	w.Reset()
	w.WriteString("\x1b[?2026h")

	for cy := 0; cy < c.Rows; cy++ {
		for cx := 0; cx < c.Cols; cx++ {
			var sum float32
			for dy := 0; dy < 4; dy++ {
				row := (cy*4 + dy) * c.DW
				sum += c.L[row+cx*2] + c.L[row+cx*2+1]
			}
			c.energy[cy*c.Cols+cx] = sum / 4
		}
	}

	lastFg, lastBg := -1, -2 // -1 = default bg sentinel, -2 = unset
	for cy := 0; cy < c.Rows; cy++ {
		w.WriteString("\x1b[")
		w.WriteString(strconv.Itoa(cy + 1))
		w.WriteString(";1H")
		for cx := 0; cx < c.Cols; cx++ {
			i := cy*c.Cols + cx
			bits, peak := c.cellBits(cx, cy)

			// Bloom: an ember-coloured cell background. Soft mode only lights cells
			// that hold stroke dots, so empty space never shows cell-shaped blocks;
			// full mode also blurs into neighbours for a wider (blockier) halo.
			var g float32
			// Only cells a line passes through glow; sparks stay bare points of
			// light instead of each dragging a cell-sized block along.
			heated := c.energy[i] > 0.04 || (c.glyph[i] != 0 && !c.glyphFixed[i])
			switch {
			case c.Glow == GlowSoft && heated:
				g = min(c.energy[i]*0.45+c.glyphV[i]*0.12*b2f(c.glyph[i] != 0), 0.28)
			case c.Glow == GlowFull:
				g = c.energy[i] * 0.4
				for _, d := range [][3]int{{-1, 0, 12}, {1, 0, 12}, {0, -1, 12}, {0, 1, 12}, {-1, -1, 3}, {1, -1, 3}, {-1, 1, 3}, {1, 1, 3}} {
					nx, ny := cx+d[0], cy+d[1]
					if nx >= 0 && ny >= 0 && nx < c.Cols && ny < c.Rows {
						g += c.energy[ny*c.Cols+nx] * float32(d[2]) / 100
					}
				}
				if c.glyph[i] != 0 && !c.glyphFixed[i] {
					g += c.glyphV[i] * 0.15
				}
				g = min(g*0.6, 0.42)
			}
			g *= 1 - c.Fizzle*0.7
			bg := -1
			if g >= 0.05 {
				bg = int(220*g)<<16 | int(55*g)<<8 | int(6*g)
			}
			if bg != lastBg {
				if bg < 0 {
					w.WriteString("\x1b[49m")
				} else {
					writeColor(w, 48, bg)
				}
				lastBg = bg
			}

			ch, fg := ' ', lastFg
			switch {
			case c.glyph[i] != 0 && c.glyphFixed[i]:
				ch, fg = c.glyph[i], rgbInt(c.glyphC[i])
			case c.glyph[i] != 0:
				ch, fg = c.glyph[i], rgbInt(c.heat(c.glyphV[i]))
			case bits != 0:
				ch, fg = 0x2800+bits, rgbInt(c.heat(peak))
			}
			if fg != lastFg && fg >= 0 {
				writeColor(w, 38, fg)
				lastFg = fg
			}
			w.WriteRune(ch)
		}
	}
	w.WriteString("\x1b[?2026l")
	return w.Bytes()
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

func rgbInt(c RGB) int { return int(c.R)<<16 | int(c.G)<<8 | int(c.B) }

func writeColor(w *bytes.Buffer, layer, rgb int) {
	w.WriteString("\x1b[")
	w.WriteString(strconv.Itoa(layer))
	w.WriteString(";2;")
	w.WriteString(strconv.Itoa(rgb >> 16 & 0xff))
	w.WriteByte(';')
	w.WriteString(strconv.Itoa(rgb >> 8 & 0xff))
	w.WriteByte(';')
	w.WriteString(strconv.Itoa(rgb & 0xff))
	w.WriteByte('m')
}
