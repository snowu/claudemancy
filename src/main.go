// claudemancy: draw a sigil in the terminal, get back the Claude skill it invokes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Result struct {
	Spell     string  `json:"spell,omitempty"`
	Skill     string  `json:"skill,omitempty"`
	Args      string  `json:"args,omitempty"`
	Score     float64 `json:"score,omitempty"`
	Cancelled bool    `json:"cancelled,omitempty"`
}

func main() {
	out := flag.String("out", "", "write the result JSON here (file or fifo) instead of stdout")
	bookFlag := flag.String("spellbook", "", "spellbook path (default ~/.config/claudemancy/spellbook.json, else built-in)")
	train := flag.String("train", "", "inscribe new templates for this spell/skill name")
	demo := flag.String("demo", "", "auto-draw a built-in shape and cast it")
	list := flag.Bool("list", false, "list spells and built-in shapes")
	t0 := flag.Int64("t0", 0, "launch timestamp (unix ns) to measure popup latency")
	latencyLog := flag.String("latency-log", "", "append popup latency (ms) to this file")
	flag.Parse()

	// Open the result sink first: a reader blocked on a fifo then sees EOF if we die.
	sink := os.Stdout
	if *out != "" {
		f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		sink = f
	}

	book, bookPath, err := LoadSpellbook(*bookFlag)
	if err != nil {
		fatal(fmt.Errorf("spellbook %s: %w", bookPath, err))
	}
	if *list {
		if _, err := os.Stat(bookPath); err != nil {
			bookPath = "built-in default; --train or create " + bookPath + " to customise"
		}
		PrintList(book, bookPath)
		return
	}
	if *demo != "" && shapes[*demo] == nil {
		fatal(fmt.Errorf("unknown shape %q; have %v", *demo, shapeNames()))
	}

	term, err := OpenTerm()
	if err != nil {
		fatal(err)
	}
	app := newApp(term, book, bookPath, *train, *demo)
	app.t0, app.latencyLog = *t0, *latencyLog
	func() {
		defer func() {
			if r := recover(); r != nil {
				term.Leave()
				panic(r)
			}
		}()
		app.Run()
	}()
	term.Leave()

	data, _ := json.Marshal(app.result)
	sink.Write(append(data, '\n'))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "claudemancy:", err)
	os.Exit(1)
}

type state int

const (
	stDraw state = iota
	stFizzle
	stCast
	stDone
)

type Spark struct{ x, y, vx, vy, life, max float64 }

type App struct {
	term     *Term
	cv       *Canvas
	pixel    bool
	book     *Spellbook
	bookPath string
	rec      *Recognizer
	train    string
	rng      *rand.Rand

	strokes     [][]Pt
	grimoire    bool
	sigils      [][]Pt // cached for the grimoire panel
	drawing     bool
	lastRelease float64
	sparks      []Spark
	demo        []Pt // auto-drawn sigil for --demo
	demoI       int

	st         state
	stT, now   float64
	msg        string
	msgUntil   float64
	latency    string
	t0         int64
	latencyLog string

	spell               Spell
	mandala             *Mandala
	fromX, fromY, fromR float64
	toX, toY, toR       float64
	burst               bool
	result              Result
}

func newApp(t *Term, book *Spellbook, path, train, demo string) *App {
	a := &App{term: t, book: book, bookPath: path, rec: book.Recognizer(), train: train,
		rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 7)), result: Result{Cancelled: true},
		grimoire: demo == "", sigils: book.Sigils()}
	a.resize()
	if demo != "" {
		w, h := a.cv.WorldW(), a.cv.WorldH()
		size := math.Min(w, h) * 0.5
		for _, p := range shapes[demo]() {
			a.demo = append(a.demo, Pt{w/2 + (p.X-0.5)*size + a.rng.NormFloat64()*0.6, h/2 + (p.Y-0.5)*size + a.rng.NormFloat64()*0.6, p.ID})
		}
	}
	return a
}

func (a *App) resize() {
	aspect := 1.0
	if a.term.CellW > 0 {
		aspect = (a.term.CellH / 4) / (a.term.CellW / 2)
	}
	a.cv = NewCanvas(a.term.Cols, a.term.Rows, aspect)
	a.cv.Glow = a.book.Glow
}

func (a *App) Run() {
	a.term.Enter()
	in := make(chan []byte, 16)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				close(in)
				return
			}
			in <- append([]byte(nil), buf[:n]...)
		}
	}()
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	tick := time.NewTicker(time.Second / 60)
	defer tick.Stop()

	a.draw()
	if a.t0 > 0 {
		ms := (time.Now().UnixNano() - a.t0) / 1e6
		a.latency = fmt.Sprintf("⚡%dms", ms)
		if a.latencyLog != "" {
			if f, err := os.OpenFile(a.latencyLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				fmt.Fprintln(f, ms)
				f.Close()
			}
		}
	}

	last := time.Now()
	var pending []byte
	for a.st != stDone {
		select {
		case b, ok := <-in:
			if !ok {
				a.st = stDone
				continue
			}
			var evs []Event
			evs, pending = parseInput(append(pending, b...))
			for _, e := range evs {
				a.handle(e)
			}
		case <-winch:
			a.term.Resize()
			a.resize()
		case now := <-tick.C:
			a.update(now.Sub(last).Seconds())
			last = now
			a.draw()
		}
	}
}

func (a *App) handle(e Event) {
	switch e.Kind {
	case evPixelMode:
		a.pixel = e.On
	case evKey:
		switch {
		case e.Key == keyEsc || e.Key == 'q' || e.Key == 3:
			a.result = Result{Cancelled: true}
			a.st = stDone
		case a.st == stCast:
			a.st = stDone // any key skips the animation
		case e.Key == '\t':
			a.grimoire = !a.grimoire
			if a.grimoire && !grimoireFits(a.cv) {
				a.say("the window is too small for the grimoire — try /spells", 2)
			}
		case e.Key == '\r' && a.train != "":
			a.inscribe()
		case e.Key == '\r':
			a.tryCast()
		case e.Key == 0x7f || e.Key == 8 || e.Key == 'c' || e.Key == 21:
			a.strokes = nil
		}
	case evMouse:
		if a.st != stDraw || e.Btn&64 != 0 {
			return
		}
		btn, motion := e.Btn&3, e.Btn&32 != 0
		x, y := a.world(e)
		switch {
		case btn == 2 && !motion && !e.Release:
			a.strokes = nil
		case btn == 0 && e.Release:
			a.drawing = false
			a.lastRelease = a.now
		case btn == 0 && !motion:
			a.drawing = true
			a.strokes = append(a.strokes, nil)
			a.addPoint(x, y)
		case btn == 0 && a.drawing:
			a.addPoint(x, y)
		}
	}
}

func (a *App) world(e Event) (float64, float64) {
	var dx, dy float64
	if a.pixel && a.term.CellW > 0 {
		dx = float64(e.X) / a.term.CellW * 2
		dy = float64(e.Y) / a.term.CellH * 4
	} else {
		dx = float64(e.X-1)*2 + 1
		dy = float64(e.Y-1)*4 + 2
	}
	return dx, dy * a.cv.Aspect
}

func (a *App) addPoint(x, y float64) {
	s := &a.strokes[len(a.strokes)-1]
	if n := len(*s); n > 0 && math.Hypot((*s)[n-1].X-x, (*s)[n-1].Y-y) < 0.6 {
		return
	}
	*s = append(*s, Pt{x, y, len(a.strokes) - 1})
	for range 3 {
		a.spark(x, y, 8+a.rng.Float64()*35, 0.2+a.rng.Float64()*0.45)
	}
}

func (a *App) spark(x, y, speed, life float64) {
	if len(a.sparks) > 2500 {
		return
	}
	ang := a.rng.Float64() * 2 * math.Pi
	a.sparks = append(a.sparks, Spark{x, y, math.Cos(ang) * speed, math.Sin(ang) * speed, life, life})
}

func (a *App) strokesReach(wx float64) bool {
	for _, s := range a.strokes {
		for _, p := range s {
			if p.X >= wx-2 {
				return true
			}
		}
	}
	return false
}

func (a *App) points() []Pt {
	var pts []Pt
	for _, s := range a.strokes {
		pts = append(pts, s...)
	}
	return pts
}

func (a *App) say(msg string, secs float64) { a.msg, a.msgUntil = msg, a.now+secs }

func (a *App) tryCast() {
	pts := a.points()
	if len(pts) < 6 {
		a.strokes = nil
		return
	}
	m := a.rec.Recognize(pts)
	if !m.Confident(a.book.MaxDistance) {
		a.st, a.stT = stFizzle, 0
		if m.Spell >= 0 && m.Dist < a.book.MaxDistance*1.25 {
			a.say("the spell fizzles… (it almost became "+a.book.Spells[m.Spell].Name+")", 1.6)
		} else {
			a.say("the spell fizzles…", 1.2)
		}
		return
	}

	a.spell = a.book.Spells[m.Spell]
	a.result = Result{Spell: a.spell.Name, Skill: a.spell.Skill, Args: a.spell.Args, Score: math.Round(m.Score()*100) / 100}
	a.mandala = NewMandala(a.spell.Name, a.book.Glyphs)
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range pts {
		minX, minY, maxX, maxY = math.Min(minX, p.X), math.Min(minY, p.Y), math.Max(maxX, p.X), math.Max(maxY, p.Y)
		a.spark(p.X, p.Y, 15+a.rng.Float64()*50, 0.3+a.rng.Float64()*0.5)
	}
	a.fromX, a.fromY = (minX+maxX)/2, (minY+maxY)/2
	a.fromR = math.Max(math.Max(maxX-minX, maxY-minY)/2, 6)
	w, h := a.cv.WorldW(), a.cv.WorldH()
	a.toX, a.toY = w/2, h/2-4*a.cv.Aspect // leave room for the label below
	a.toR = math.Min(w/2*0.92, h/2-4*a.cv.Aspect*2.5)
	a.st, a.stT, a.burst = stCast, 0, false
}

func (a *App) inscribe() {
	pts := a.points()
	if len(pts) < 6 {
		return
	}
	n := a.book.Inscribe(a.train, pts)
	if err := a.book.Save(a.bookPath); err != nil {
		a.say("could not save: "+err.Error(), 3)
		return
	}
	a.rec, a.sigils = a.book.Recognizer(), a.book.Sigils()
	for _, p := range pts {
		a.spark(p.X, p.Y, 10+a.rng.Float64()*30, 0.3+a.rng.Float64()*0.4)
	}
	a.strokes = nil
	a.say(fmt.Sprintf("inscribed ✓  %s now has %d trained sigil(s) — draw it again to strengthen it", a.train, n), 2.5)
}

func (a *App) update(dt float64) {
	dt = math.Min(dt, 0.05)
	a.now += dt
	a.stT += dt

	live := a.sparks[:0]
	for _, s := range a.sparks {
		s.life -= dt
		if s.life <= 0 {
			continue
		}
		s.vy += 55 * dt
		s.vx *= 1 - 1.8*dt
		s.vy *= 1 - 1.8*dt
		s.x += s.vx * dt
		s.y += s.vy * dt
		live = append(live, s)
	}
	a.sparks = live

	if a.demo != nil && a.st == stDraw {
		a.drawing = true
		n := min(int(a.now/0.7*float64(len(a.demo))), len(a.demo))
		for ; a.demoI < n; a.demoI++ {
			p := a.demo[a.demoI]
			if len(a.strokes) <= p.ID {
				a.strokes = append(a.strokes, nil)
			}
			a.addPoint(p.X, p.Y)
		}
		if a.demoI == len(a.demo) {
			a.demo, a.drawing, a.lastRelease = nil, false, a.now
		}
	}

	switch a.st {
	case stDraw:
		if !a.drawing && a.demo == nil && a.train == "" && len(a.strokes) > 0 && a.now-a.lastRelease > 0.55 {
			a.tryCast()
		}
	case stFizzle:
		if a.stT > 0.75 {
			a.strokes, a.st = nil, stDraw
		}
	case stCast:
		if a.stT >= 1.45 && !a.burst {
			a.burst = true
			for range 320 {
				a.spark(a.toX, a.toY, 30+a.rng.Float64()*160, 0.25+a.rng.Float64()*0.45)
			}
		}
		if a.stT >= 1.9 {
			a.st = stDone
		}
	}
}

func clamp01(v float64) float64 { return math.Min(math.Max(v, 0), 1) }

func (a *App) draw() {
	c := a.cv
	c.Clear()
	c.Fizzle = 0
	if a.st == stFizzle {
		c.Fizzle = float32(clamp01(a.stT / 0.25))
	}

	alpha := 1.0
	if a.st == stCast {
		alpha = 1 - clamp01(a.stT/0.35)
	}
	if alpha > 0 {
		for _, s := range a.strokes {
			for i := 1; i < len(s); i++ {
				v := float32(alpha * (0.88 + 0.12*math.Sin(a.now*18+float64(i)*0.45)))
				c.Line(s[i-1].X, s[i-1].Y, s[i].X, s[i].Y, v)
			}
			if len(s) == 1 {
				c.Plot(s[0].X, s[0].Y, float32(alpha))
			}
		}
		if a.drawing && len(a.strokes) > 0 {
			if s := a.strokes[len(a.strokes)-1]; len(s) > 0 {
				tip := s[len(s)-1]
				c.Circle(tip.X, tip.Y, 1.5, 1.4)
			}
		}
	}

	for _, s := range a.sparks {
		c.Plot(s.x, s.y, float32(s.life/s.max*1.25))
	}

	if a.st == stCast {
		t := a.stT
		k := 1 - math.Pow(1-clamp01(t/0.4), 3)
		cx, cy := a.fromX+(a.toX-a.fromX)*k, a.fromY+(a.toY-a.fromY)*k
		R := a.fromR + (a.toR-a.fromR)*k
		v := float32(k)
		if t > 0.4 {
			v = float32(1 + 0.1*math.Sin(t*9))
		}
		if t > 1.15 {
			k2 := math.Pow(clamp01((t-1.15)/0.3), 3)
			R *= 1 - k2
			v = float32(1 + 0.5*k2)
		}
		if t < 1.45 {
			a.mandala.Draw(c, cx, cy, R, t, v)
		}
		if t > 0.25 && t < 1.45 {
			row := int((a.toY+a.toR)/c.Aspect/4) + 1
			col := c.heat(min(v, 1.1))
			title := "⟡  " + a.spell.Name + "  ⟡"
			sub := a.spell.Invocation()
			c.Text((c.Cols-len([]rune(title)))/2, min(row, c.Rows-2), title, col)
			c.Text((c.Cols-len([]rune(sub)))/2, min(row+1, c.Rows-1), sub, RGB{col.R / 2, col.G / 2, col.B / 2})
		}
	}

	// The panel steps aside while a sigil reaches into it, so it never hides strokes.
	if a.grimoire && a.st != stCast && !a.strokesReach(grimoireLeft(c)) {
		drawGrimoire(c, a.book, a.sigils)
	}

	hud := RGB{150, 68, 18}
	if a.train != "" {
		c.Text(1, 0, "✦ inscribing "+a.train+"  ·  draw the sigil  ·  ⏎ save  ·  ⌫ clear  ·  ⇥ grimoire  ·  esc done", hud)
	} else if a.st != stCast {
		c.Text(1, 0, "✦ claudemancy  ·  draw a sigil  ·  ⌫ clear  ·  ⇥ grimoire  ·  esc dismiss", hud)
	}
	if a.latency != "" && a.now < 2.5 {
		c.Text(c.Cols-len([]rune(a.latency))-1, c.Rows-1, a.latency, hud)
	}
	if a.now < a.msgUntil {
		c.Text((c.Cols-len([]rune(a.msg)))/2, c.Rows-1, a.msg, RGB{200, 110, 40})
	}
	os.Stdout.Write(c.Render())
}
