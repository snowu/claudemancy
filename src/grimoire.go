package main

import (
	"fmt"
	"math"
	"strings"
)

// Sigil returns the shape a spell is drawn as: its built-in shape, else its
// first trained template.
func (b *Spellbook) Sigil(i int) []Pt {
	s := b.Spells[i]
	if gen, ok := shapes[s.Shape]; ok {
		return gen()
	}
	if len(s.Templates) > 0 {
		pts := make([]Pt, len(s.Templates[0]))
		for j, p := range s.Templates[0] {
			pts[j] = Pt{p[0], p[1], int(p[2])}
		}
		return pts
	}
	return nil
}

func (s Spell) Invocation() string {
	if s.Args != "" {
		return "/" + s.Skill + " " + s.Args
	}
	return "/" + s.Skill
}

// drawSigil fits pts into the world-space box (x, y, w, h), keeping proportions.
func drawSigil(c *Canvas, pts []Pt, x, y, w, h float64, v float32) {
	if len(pts) < 2 {
		return
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	scale := math.Min(w/math.Max(maxX-minX, 1e-9), h/math.Max(maxY-minY, 1e-9))
	ox := x + (w-(maxX-minX)*scale)/2
	oy := y + (h-(maxY-minY)*scale)/2
	for i := 1; i < len(pts); i++ {
		if pts[i].ID == pts[i-1].ID {
			c.Line(ox+(pts[i-1].X-minX)*scale, oy+(pts[i-1].Y-minY)*scale,
				ox+(pts[i].X-minX)*scale, oy+(pts[i].Y-minY)*scale, v)
		}
	}
}

const grimoireCols = 34

// drawGrimoire lists the spellbook down the right edge: sigil, name, invocation.
func drawGrimoire(c *Canvas, b *Spellbook) {
	if c.Cols < grimoireCols*2 || c.Rows < 8 {
		return
	}
	col0 := c.Cols - grimoireCols
	title := RGB{170, 80, 22}
	dim := RGB{120, 58, 18}
	c.Text(col0, 2, "✦ grimoire", title)
	row := 4
	for i, s := range b.Spells {
		if row+3 > c.Rows-1 {
			c.Text(col0, row, fmt.Sprintf("… %d more (claudemancy --list)", len(b.Spells)-i), dim)
			break
		}
		// 5x3-cell sigil box, inset by a dot so strokes don't touch the edge.
		x, y := float64(col0*2)+1, float64(row*4)*c.Aspect+1
		drawSigil(c, b.Sigil(i), x, y, 8, 12*c.Aspect-2, 0.3)
		c.Text(col0+6, row, truncate(s.Name, grimoireCols-7), title)
		c.Text(col0+6, row+1, truncate(s.Invocation(), grimoireCols-7), dim)
		row += 4
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Plain renders the dot field as uncoloured Braille lines, for non-TUI output.
func (c *Canvas) Plain() []string {
	lines := make([]string, c.Rows)
	for cy := 0; cy < c.Rows; cy++ {
		var sb strings.Builder
		for cx := 0; cx < c.Cols; cx++ {
			var bits rune
			for dy := 0; dy < 4; dy++ {
				for dx := 0; dx < 2; dx++ {
					if c.I[(cy*4+dy)*c.DW+cx*2+dx] > 0.08 {
						bits |= brailleBit[dy][dx]
					}
				}
			}
			sb.WriteRune(0x2800 + bits)
		}
		lines[cy] = sb.String()
	}
	return lines
}

// PrintList writes the spellbook with a small Braille sigil per spell.
func PrintList(b *Spellbook, path string) {
	fmt.Printf("claudemancy grimoire  (%s)\n\n", path)
	for i, s := range b.Spells {
		c := NewCanvas(5, 3, 1)
		drawSigil(c, b.Sigil(i), 0.5, 0.5, 9, 11, 1)
		art := c.Plain()
		how := s.Shape
		if n := len(s.Templates); n > 0 {
			if how != "" {
				how += " + "
			}
			how += fmt.Sprintf("%d trained", n)
		}
		fmt.Printf("  %s   %s\n", art[0], s.Name)
		fmt.Printf("  %s   %s\n", art[1], s.Invocation())
		fmt.Printf("  %s   %s\n\n", art[2], how)
	}
	fmt.Println("built-in shapes:", strings.Join(shapeNames(), " "))
}
