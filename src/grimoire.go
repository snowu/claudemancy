package main

import (
	"fmt"
	"math"
	"strings"
)

// Sigil returns the segments a spell is previewed with: its first drawing,
// else (for legacy spellbook.json spells) its built-in shape or first trained
// template.
func (b *Spellbook) Sigil(i int) []Pt {
	s := b.Spells[i]
	if len(s.Drawings) > 0 {
		return s.Drawings[0].Segments()
	}
	if gen, ok := shapes[s.Shape]; ok {
		return gen()
	}
	if len(s.Templates) > 0 {
		return templatePts(s.Templates[0])
	}
	return nil
}

// Sigils returns every spell's sigil, for callers that redraw them each frame.
func (b *Spellbook) Sigils() [][]Pt {
	out := make([][]Pt, len(b.Spells))
	for i := range b.Spells {
		out[i] = b.Sigil(i)
	}
	return out
}

// Invocation is how the grimoire shows what a spell does.
func (s Spell) Invocation() string {
	if s.Prompt != "" {
		return "✎ " + s.Prompt
	}
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
			c.FaintLine(ox+(pts[i-1].X-minX)*scale, oy+(pts[i-1].Y-minY)*scale,
				ox+(pts[i].X-minX)*scale, oy+(pts[i].Y-minY)*scale, v)
		}
	}
}

const grimoireCols = 34

func grimoireFits(c *Canvas) bool { return c.Cols >= grimoireCols*2 && c.Rows >= 8 }

// grimoireLeft is the panel's left edge in world units.
func grimoireLeft(c *Canvas) float64 { return float64((c.Cols - grimoireCols) * 2) }

// drawGrimoire lists the spellbook down the right edge: sigil, name, invocation.
func drawGrimoire(c *Canvas, b *Spellbook, sigils [][]Pt) {
	if !grimoireFits(c) {
		return
	}
	col0 := c.Cols - grimoireCols
	title := RGB{170, 80, 22}
	dim := RGB{120, 58, 18}
	c.Text(col0, 2, "✦ grimoire", title)
	row := 4
	for i, s := range b.Spells {
		if row+3 > c.Rows-1 {
			c.Text(col0, row, fmt.Sprintf("… %d more (/spells)", len(b.Spells)-i), dim)
			break
		}
		// 7x3-cell sigil box, inset by a dot so strokes don't touch the edge.
		x, y := float64(col0*2)+1, float64(row*4)*c.Aspect+1
		drawSigil(c, sigils[i], x, y, 12, 12*c.Aspect-2, 0.6)
		c.Text(col0+8, row, truncate(s.Name, grimoireCols-9), title)
		c.Text(col0+8, row+1, truncate(s.Invocation(), grimoireCols-9), dim)
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
			bits, _ := c.cellBits(cx, cy)
			sb.WriteRune(0x2800 + bits)
		}
		lines[cy] = sb.String()
	}
	return lines
}

// PrintList writes the spellbook with a small Braille sigil per spell, where
// each spell comes from, and any sigil files that failed to load.
func PrintList(b *Spellbook, dirs []SpellDir) {
	fmt.Println("claudemancy grimoire")
	fmt.Println()
	for i, s := range b.Spells {
		c := NewCanvas(8, 4, 1)
		drawSigil(c, b.Sigil(i), 0.5, 0.5, 15, 15, 1)
		art := c.Plain()
		from := s.Source
		if s.File != "" {
			from += "  " + s.File
		}
		fmt.Printf("  %s   %s\n", art[0], s.Name)
		fmt.Printf("  %s   %s\n", art[1], truncate(s.Invocation(), 72))
		fmt.Printf("  %s   %s\n", art[2], from)
		fmt.Printf("  %s\n\n", art[3])
	}
	for _, p := range b.Problems {
		fmt.Println("  ✗", p)
	}
	fmt.Println("spell folders (later ones override earlier ones by filename):")
	for _, d := range dirs {
		fmt.Printf("  %-9s %s\n", d.Label, d.Path)
	}
}
