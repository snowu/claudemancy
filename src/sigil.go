package main

// Sigil files: one spell per file, a short header and then one or more ASCII
// drawings of the sigil. Any character other than space or '.' is ink.
//
//	name: Eye of Scrutiny
//	skill: code-review
//	---
//	      #
//	     # #
//	    #   #
//	   #######
//
// More drawings of the same spell (variants the recognizer also accepts)
// follow after further "---" lines. See docs/sigils.md for the full format.

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const sigilExt = ".sigil"

// sigilCols is the width trained drawings are written at: detailed enough to
// recognise well, small enough to read and hand-edit.
const sigilCols = 24

// defaultAspect is how much taller a character cell is than it is wide in a
// typical monospace font, so ASCII drawings keep the proportions they show.
const defaultAspect = 2.0

type Drawing struct {
	Lines  []string
	Aspect float64
}

func isInk(ch rune) bool { return ch != '.' && !unicode.IsSpace(ch) }

// Points returns the drawing's ink cells as an unordered point cloud (ID -1).
func (d Drawing) Points() []Pt {
	var pts []Pt
	for row, line := range d.Lines {
		for col, ch := range []rune(line) {
			if isInk(ch) {
				pts = append(pts, Pt{float64(col), float64(row) * d.Aspect, -1})
			}
		}
	}
	return pts
}

// Strokes traces the drawing's ink into polylines, so a file is matched with
// exactly the same pipeline as a drawn stroke. It walks from line ends through
// neighbouring ink, prefers to carry straight on at crossings, and bridges
// one-cell gaps like "# # #".
func (d Drawing) Strokes() []Pt {
	type cell struct{ c, r int }
	var cells []cell
	ink := map[cell]bool{}
	for r, line := range d.Lines {
		for c, ch := range []rune(line) {
			if isInk(ch) {
				ink[cell{c, r}] = true
				cells = append(cells, cell{c, r})
			}
		}
	}
	seen := map[cell]bool{}
	open := func(p cell, reach int) []cell {
		var out []cell
		for dr := -reach; dr <= reach; dr++ {
			for dc := -reach; dc <= reach; dc++ {
				if q := (cell{p.c + dc, p.r + dr}); (dr != 0 || dc != 0) && ink[q] && !seen[q] {
					out = append(out, q)
				}
			}
		}
		return out
	}

	var pts []Pt
	for id := 0; ; id++ {
		// Start at a line end (fewest open neighbours); closed loops start anywhere.
		start, best := cell{}, -1
		for _, p := range cells {
			if n := len(open(p, 1)); !seen[p] && (best < 0 || n < best) {
				start, best = p, n
			}
		}
		if best < 0 {
			return pts
		}
		cur, dirX, dirY := start, 0.0, 0.0
		for {
			seen[cur] = true
			pts = append(pts, Pt{float64(cur.c), float64(cur.r) * d.Aspect, id})
			next := open(cur, 1)
			if len(next) == 0 {
				next = open(cur, 2)
			}
			if len(next) == 0 {
				break
			}
			var pick cell
			score := math.Inf(-1)
			for _, q := range next {
				dx, dy := float64(q.c-cur.c), float64(q.r-cur.r)*d.Aspect
				l := math.Hypot(dx, dy)
				if sc := (dx*dirX+dy*dirY)/l - 0.05*l; sc > score {
					pick, score = q, sc
				}
			}
			dx, dy := float64(pick.c-cur.c), float64(pick.r-cur.r)*d.Aspect
			l := math.Hypot(dx, dy)
			dirX, dirY, cur = dx/l, dy/l, pick
		}
	}
}

// ParseSigil reads a sigil file's contents; id is its filename stem.
func ParseSigil(id, src string) (Spell, error) {
	s := Spell{ID: id, Name: id}
	aspect := defaultAspect
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	i := 0
	header := true
	for ; i < len(lines) && header; i++ {
		l := strings.TrimSpace(lines[i])
		switch {
		case l == "---":
			header = false
			continue
		case l == "" || strings.HasPrefix(l, "//"):
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			return s, fmt.Errorf("line %d: expected `key: value` or a --- line before the drawing", i+1)
		}
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "name":
			s.Name = v
		case "skill":
			s.Skill = strings.TrimPrefix(v, "/")
		case "args":
			s.Args = v
		case "prompt":
			s.Prompt = v
		case "aspect":
			a, err := strconv.ParseFloat(v, 64)
			if err != nil || a <= 0 {
				return s, fmt.Errorf("line %d: aspect must be a positive number", i+1)
			}
			aspect = a
		case "disabled":
			s.Disabled = v == "true" || v == "yes"
		default:
			return s, fmt.Errorf("line %d: unknown key %q (want name, skill, args, prompt, aspect or disabled)", i+1, k)
		}
	}
	if header {
		return s, errors.New("missing the --- line that ends the header")
	}

	var cur []string
	flush := func() {
		if d := trimBlankLines(cur); len(d) > 0 {
			s.Drawings = append(s.Drawings, Drawing{d, aspect})
		}
		cur = nil
	}
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			flush()
			continue
		}
		cur = append(cur, strings.TrimRightFunc(lines[i], unicode.IsSpace))
	}
	flush()

	if s.Disabled {
		return s, nil
	}
	switch {
	case s.Skill == "" && s.Prompt == "":
		return s, errors.New("needs a `skill:` or a `prompt:`")
	case s.Skill != "" && s.Prompt != "":
		return s, errors.New("has both `skill:` and `prompt:`; pick one")
	case s.Args != "" && s.Skill == "":
		return s, errors.New("`args:` only applies to a `skill:`")
	case len(s.Drawings) == 0:
		return s, errors.New("has no drawing after the --- line")
	}
	for n, d := range s.Drawings {
		if len(d.Points()) < 4 {
			return s, fmt.Errorf("drawing %d has fewer than 4 ink characters", n+1)
		}
	}
	return s, nil
}

func trimBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// FormatSigil renders a spell back to the file format.
func FormatSigil(s Spell) string {
	var b strings.Builder
	field := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, v)
		}
	}
	field("name", s.Name)
	field("skill", s.Skill)
	field("args", s.Args)
	field("prompt", s.Prompt)
	if s.Disabled {
		field("disabled", "true")
	}
	if len(s.Drawings) > 0 && s.Drawings[0].Aspect != defaultAspect {
		field("aspect", strconv.FormatFloat(s.Drawings[0].Aspect, 'g', -1, 64))
	}
	for _, d := range s.Drawings {
		b.WriteString("---\n")
		for _, l := range d.Lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	if len(s.Drawings) == 0 {
		b.WriteString("---\n")
	}
	return b.String()
}

// Rasterize turns strokes into an ASCII drawing about cols characters wide.
func Rasterize(pts []Pt, cols int, aspect float64) Drawing {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		minX, minY = math.Min(minX, p.X), math.Min(minY, p.Y)
		maxX, maxY = math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	scale := float64(cols-1) / math.Max(math.Max(maxX-minX, maxY-minY), 1e-9)
	rows := int(math.Round((maxY-minY)*scale/aspect)) + 1
	grid := make([][]rune, rows)
	for r := range grid {
		grid[r] = []rune(strings.Repeat(" ", cols))
	}
	cellOf := func(p Pt) (int, int) {
		return int(math.Round((p.X - minX) * scale)), int(math.Round((p.Y - minY) * scale / aspect))
	}
	ink := func(c, r int) {
		if r >= 0 && r < rows && c >= 0 && c < cols {
			grid[r][c] = '#'
		}
	}
	// Bresenham between consecutive cells keeps lines one character wide.
	line := func(c0, r0, c1, r1 int) {
		dc, dr := abs(c1-c0), -abs(r1-r0)
		sc, sr := sign(c1-c0), sign(r1-r0)
		e := dc + dr
		for {
			ink(c0, r0)
			if c0 == c1 && r0 == r1 {
				return
			}
			if e2 := 2 * e; e2 >= dr {
				e, c0 = e+dr, c0+sc
			} else {
				e, r0 = e+dc, r0+sr
			}
		}
	}
	for i, p := range pts {
		c, r := cellOf(p)
		if p.ID < 0 || i == 0 || pts[i-1].ID != p.ID {
			ink(c, r)
			continue
		}
		pc, pr := cellOf(pts[i-1])
		line(pc, pr, c, r)
	}
	lines := make([]string, rows)
	for r := range grid {
		lines[r] = strings.TrimRight(string(grid[r]), " ")
	}
	return Drawing{trimBlankLines(lines), aspect}
}

// SpellDir is one layer of sigil files; later layers override earlier ones.
type SpellDir struct {
	Path, Label string
}

// SpellDirs lists the layers: the plugin's built-ins, the user's, the project's.
func SpellDirs(project string) []SpellDir {
	var dirs []SpellDir
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			dirs = append(dirs, SpellDir{filepath.Join(filepath.Dir(exe), "..", "spells"), "built-in"})
		}
	}
	dirs = append(dirs, SpellDir{userSpellsDir(), "user"})
	if project != "" {
		dirs = append(dirs, SpellDir{filepath.Join(project, ".claude", "claudemancy", "spells"), "project"})
	}
	return dirs
}

func userSpellsDir() string { return filepath.Join(filepath.Dir(userSpellbookPath()), "spells") }

// LoadSpellDirs reads every *.sigil file, letting later dirs replace earlier
// spells with the same filename. Broken files are reported, not fatal.
func LoadSpellDirs(dirs []SpellDir) (spells []Spell, problems []error) {
	byID := map[string]Spell{}
	for _, d := range dirs {
		files, _ := filepath.Glob(filepath.Join(d.Path, "*"+sigilExt))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				problems = append(problems, err)
				continue
			}
			id := strings.TrimSuffix(filepath.Base(f), sigilExt)
			s, err := ParseSigil(id, string(data))
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", f, err))
				continue
			}
			s.File, s.Source = f, d.Label
			byID[id] = s
		}
	}
	for _, s := range byID {
		if !s.Disabled {
			spells = append(spells, s)
		}
	}
	sort.Slice(spells, func(i, j int) bool { return spells[i].ID < spells[j].ID })
	return spells, problems
}

func abs(v int) int { return max(v, -v) }

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func titleize(id string) string {
	words := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
