package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// Every cast attempt is logged locally (strokes plus how it scored) so the
// recognizer can be tuned on real drawings with --replay, not just simulated
// ones. Nothing leaves the machine.

type Attempt struct {
	Time  time.Time    `json:"t"`
	Pts   [][3]float64 `json:"pts"` // [x, y, stroke]
	Best  string       `json:"best,omitempty"`
	Dist  float64      `json:"dist"`
	Ratio float64      `json:"ratio"`
	Cast  bool         `json:"cast"`
}

const maxAttemptLog = 2 << 20 // bytes; the log rotates to .1 past this

func attemptLogPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "claudemancy", "attempts.jsonl")
}

// logAttempt appends one attempt. Failures are ignored: logging must never
// get in the way of casting.
func logAttempt(b *Spellbook, pts []Pt, m Match) {
	path := attemptLogPath()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxAttemptLog {
		os.Rename(path, path+".1")
	}
	a := Attempt{Time: time.Now(), Dist: round2(m.Dist), Ratio: round2(m.Ratio), Cast: m.Confident(b.MaxDistance)}
	if m.Spell >= 0 {
		a.Best = b.Spells[m.Spell].ID
	}
	for _, p := range pts {
		a.Pts = append(a.Pts, [3]float64{math.Round(p.X*10) / 10, math.Round(p.Y*10) / 10, float64(p.ID)})
	}
	data, err := json.Marshal(a)
	if err != nil {
		return
	}
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		f.Write(append(data, '\n'))
		f.Close()
	}
}

func round2(v float64) float64 {
	if math.IsInf(v, 0) {
		return 99
	}
	return math.Round(v*100) / 100
}

// Replay re-scores every logged attempt against the current spells and shows
// how many would cast under a range of thresholds.
func Replay(b *Spellbook) {
	f, err := os.Open(attemptLogPath())
	if err != nil {
		fmt.Println("no attempts logged yet:", err)
		return
	}
	defer f.Close()
	r := b.Recognizer()
	var ms []Match
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var a Attempt
		if json.Unmarshal(sc.Bytes(), &a) != nil || len(a.Pts) < 2 {
			continue
		}
		pts := make([]Pt, len(a.Pts))
		for i, p := range a.Pts {
			pts[i] = Pt{p[0], p[1], int(p[2])}
		}
		m := r.Recognize(pts)
		ms = append(ms, m)
		name := func(i int) string {
			if i < 0 {
				return "-"
			}
			return b.Spells[i].ID
		}
		mark := "fizzle"
		if m.Confident(b.MaxDistance) {
			mark = "CAST"
		}
		fmt.Printf("%s  %-6s best %-26s dist %.2f  ratio %.2f  runner-up %s\n",
			a.Time.Format("15:04:05"), mark, name(m.Spell), m.Dist, m.Ratio, name(m.Second))
	}
	if len(ms) == 0 {
		return
	}
	fmt.Printf("\n%d attempts; how many would cast (current: max_distance %.2f, ratio < 0.9):\n", len(ms), b.MaxDistance)
	fmt.Printf("%-10s", "dist\\ratio")
	ratios := []float64{0.85, 0.9, 0.95}
	for _, rt := range ratios {
		fmt.Printf("  < %.2f", rt)
	}
	fmt.Println()
	for _, d := range []float64{1.22, 1.3, 1.35, 1.4, 1.5, 1.6} {
		fmt.Printf("< %-8.2f", d)
		for _, rt := range ratios {
			n := 0
			for _, m := range ms {
				if m.Spell >= 0 && m.Dist < d && m.Ratio < rt {
					n++
				}
			}
			fmt.Printf("  %6d", n)
		}
		fmt.Println()
	}
}
