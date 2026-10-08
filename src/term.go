package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// Term owns the raw-mode tty: size, pixel metrics and the escape modes we enable.
type Term struct {
	fd           int
	saved        unix.Termios
	Cols, Rows   int
	CellW, CellH float64 // pixels per cell; 0 when the terminal doesn't report pixel size
}

func OpenTerm() (*Term, error) {
	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetTermios(fd, ioctlGetTermios)
	if err != nil {
		return nil, err
	}
	t := &Term{fd: fd, saved: *old}
	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, ioctlSetTermios, &raw); err != nil {
		return nil, err
	}
	t.Resize()
	return t, nil
}

func (t *Term) Resize() {
	ws, err := unix.IoctlGetWinsize(t.fd, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		t.Cols, t.Rows, t.CellW, t.CellH = 80, 24, 0, 0
		return
	}
	t.Cols, t.Rows = int(ws.Col), int(ws.Row)
	t.CellW, t.CellH = 0, 0
	if ws.Xpixel > 0 && ws.Ypixel > 0 {
		t.CellW = float64(ws.Xpixel) / float64(ws.Col)
		t.CellH = float64(ws.Ypixel) / float64(ws.Row)
	}
}

// Enter switches to the alt screen and turns on mouse tracking. When pixel
// metrics are known it also requests SGR-pixel mouse reports (mode 1016) and
// asks the terminal (DECRQM) whether it actually honoured that.
func (t *Term) Enter() {
	s := "\x1b[?1049h\x1b[?25l\x1b[2J\x1b[?1000h\x1b[?1002h\x1b[?1006h"
	if t.CellW > 0 {
		s += "\x1b[?1016h\x1b[?1016$p"
	}
	os.Stdout.WriteString(s)
}

func (t *Term) Leave() {
	os.Stdout.WriteString("\x1b[?1016l\x1b[?1006l\x1b[?1002l\x1b[?1000l\x1b[0m\x1b[2J\x1b[?25h\x1b[?1049l")
	unix.IoctlSetTermios(t.fd, ioctlSetTermios, &t.saved)
}
