package main

import (
	"strconv"
	"strings"
)

type EventKind int

const (
	evKey EventKind = iota
	evMouse
	evPixelMode // DECRPM reply for mode 1016
)

const keyEsc = 0x1b

type Event struct {
	Kind    EventKind
	Key     byte
	Btn     int // SGR button code (low bits = button, 32 = motion, 64 = wheel)
	X, Y    int // 1-based cells, or pixels when pixel mode is on
	Release bool
	On      bool // evPixelMode: terminal confirmed mode 1016
}

// parseInput consumes as many complete events from buf as it can and returns
// the unconsumed tail (a partial escape sequence split across reads).
func parseInput(buf []byte) ([]Event, []byte) {
	var evs []Event
	for len(buf) > 0 {
		if buf[0] != 0x1b {
			evs = append(evs, Event{Kind: evKey, Key: buf[0]})
			buf = buf[1:]
			continue
		}
		if len(buf) == 1 {
			evs = append(evs, Event{Kind: evKey, Key: keyEsc})
			return evs, nil
		}
		if buf[1] != '[' {
			buf = buf[2:] // alt+key: ignore
			continue
		}
		end := -1
		for i := 2; i < len(buf); i++ {
			if buf[i] >= 0x40 && buf[i] <= 0x7e {
				end = i
				break
			}
		}
		if end < 0 {
			return evs, buf
		}
		body, final := string(buf[2:end]), buf[end]
		buf = buf[end+1:]
		switch {
		case strings.HasPrefix(body, "<") && (final == 'M' || final == 'm'):
			f := strings.Split(body[1:], ";")
			if len(f) != 3 {
				continue
			}
			b, _ := strconv.Atoi(f[0])
			x, _ := strconv.Atoi(f[1])
			y, _ := strconv.Atoi(f[2])
			evs = append(evs, Event{Kind: evMouse, Btn: b, X: x, Y: y, Release: final == 'm'})
		case strings.HasPrefix(body, "?1016;") && final == 'y':
			v := strings.TrimSuffix(body[len("?1016;"):], "$")
			evs = append(evs, Event{Kind: evPixelMode, On: v == "1" || v == "3"})
		}
	}
	return evs, nil
}
