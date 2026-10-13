package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	cReset  = "\x1b[0m"
	cBold   = "\x1b[1m"
	cDim    = "\x1b[2m"
	cRed    = "\x1b[31m"
	cGreen  = "\x1b[32m"
	cYellow = "\x1b[33m"
	cCyan   = "\x1b[36m"
)

var colorEnabled bool

func Init() {
	enableVirtualTerminal()
	colorEnabled = os.Getenv("NO_COLOR") == ""
}

func paint(c, s string) string {
	if !colorEnabled {
		return s
	}
	return c + s + cReset
}

func Bold(s string) string { return paint(cBold, s) }

func Dim(s string) string { return paint(cDim, s) }

func Cyan(s string) string { return paint(cCyan, s) }

func Banner() {
	line := strings.Repeat("=", 56)
	fmt.Println(paint(cCyan+cBold, line))
	fmt.Println(paint(cCyan+cBold, "  Skindl  -  загрузчик скинов Rust Workshop"))
	fmt.Println(paint(cDim, "  github.com/Wakanda1519"))
	fmt.Println(paint(cCyan+cBold, line))
	fmt.Println()
}

func Step(n, total int, msg string) {
	fmt.Printf("%s %s\n", paint(cCyan+cBold, fmt.Sprintf("[%d/%d]", n, total)), paint(cBold, msg))
}

func OK(format string, args ...any) {
	fmt.Printf("  %s %s\n", paint(cGreen, "OK"), fmt.Sprintf(format, args...))
}

func Warn(format string, args ...any) {
	fmt.Printf("  %s %s\n", paint(cYellow, "!"), fmt.Sprintf(format, args...))
}

func Err(format string, args ...any) {
	fmt.Printf("  %s %s\n", paint(cRed, "x"), fmt.Sprintf(format, args...))
}

func Note(format string, args ...any) {
	fmt.Printf("  %s %s\n", paint(cDim, " "), paint(cDim, fmt.Sprintf(format, args...)))
}

func Pause() {
	if !ownsConsole() {
		return
	}
	fmt.Print(paint(cDim, "\nНажмите Enter, чтобы закрыть... "))
	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

type Progress struct {
	mu     sync.Mutex
	total  int
	done   int
	failed int
	label  string
	active bool
}

func NewProgress(total int) *Progress {
	return &Progress{total: total}
}

func (p *Progress) Start() {
	p.mu.Lock()
	p.active = true
	p.mu.Unlock()
	p.render()
}

func (p *Progress) SetLabel(label string) {
	p.mu.Lock()
	p.label = label
	p.mu.Unlock()
	p.render()
}

func (p *Progress) Add(n int) {
	p.mu.Lock()
	p.done += n
	p.mu.Unlock()
	p.render()
}

func (p *Progress) Fail() {
	p.mu.Lock()
	p.failed++
	p.mu.Unlock()
	p.render()
}

func (p *Progress) AddFailed() {
	p.mu.Lock()
	p.done++
	p.failed++
	p.mu.Unlock()
	p.render()
}

func (p *Progress) Finish() {
	p.mu.Lock()
	p.active = false
	p.mu.Unlock()
	p.render()
	fmt.Println()
}

func (p *Progress) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active || p.total <= 0 {
		return
	}

	const width = 32
	ratio := float64(p.done) / float64(p.total)
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio*float64(width) + 0.5)

	var b strings.Builder
	b.WriteString("\r")
	if colorEnabled {
		b.WriteString(cGreen)
	}
	b.WriteString(strings.Repeat("#", filled))
	if colorEnabled {
		b.WriteString(cDim)
	}
	b.WriteString(strings.Repeat("-", width-filled))
	if colorEnabled {
		b.WriteString(cReset)
	}
	fmt.Fprintf(&b, " %d/%d (%.1f%%)", p.done, p.total, ratio*100)
	ok := p.done - p.failed
	okStr := fmt.Sprintf("  успешно: %d", ok)
	if colorEnabled {
		okStr = cGreen + okStr + cReset
	}
	b.WriteString(okStr)
	if p.failed > 0 {
		failed := fmt.Sprintf("  ошибок: %d", p.failed)
		if colorEnabled {
			failed = cRed + failed + cReset
		}
		b.WriteString(failed)
	}
	if p.label != "" {
		b.WriteString("  ")
		b.WriteString(p.label)
	}
	b.WriteString("\x1b[K")
	fmt.Print(b.String())
}
