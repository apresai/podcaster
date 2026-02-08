package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/mattn/go-isatty"
)

// BarRenderer draws a two-line progress display (status + bar) on a TTY,
// or prints timestamped single lines on a non-TTY.
type BarRenderer struct {
	out       io.Writer
	start     time.Time
	isTTY     bool
	width     int
	lastEvent Event
	lines     int // number of lines currently written (for TTY overwrite)
}

// NewBarRenderer creates a renderer that writes to out.
// It auto-detects TTY mode and terminal width.
func NewBarRenderer(out *os.File) *BarRenderer {
	tty := isatty.IsTerminal(out.Fd()) || isatty.IsCygwinTerminal(out.Fd())

	width := 80
	if tty {
		if w, _, err := term.GetSize(out.Fd()); err == nil && w > 0 {
			width = w
		}
	}

	return &BarRenderer{
		out:   out,
		start: time.Now(),
		isTTY: tty,
		width: width,
	}
}

// Handle processes a progress event. It satisfies the Callback type.
func (r *BarRenderer) Handle(e Event) {
	e.Elapsed = time.Since(r.start)

	// StageComplete is always 100% regardless of the calculated percent.
	if e.Stage == StageComplete {
		e.Percent = 1.0
	}

	r.lastEvent = e

	if r.isTTY {
		r.renderTTY(e)
	} else {
		r.renderPlain(e)
	}
}

// Finish clears the progress display and prints a final summary.
func (r *BarRenderer) Finish() {
	e := r.lastEvent
	if r.isTTY && r.lines > 0 {
		// Clear the progress lines
		r.clearLines()
	}

	if e.Error != nil {
		fmt.Fprintf(r.out, "\n  Error: %v\n", e.Error)
		return
	}

	if e.Stage == StageComplete && e.OutputFile != "" {
		// Print final summary
		if e.Duration != "" {
			fmt.Fprintf(r.out, "\n  Episode saved to %s (%s, %.1f MB)\n", e.OutputFile, e.Duration, e.SizeMB)
		} else if e.SizeMB > 0 {
			fmt.Fprintf(r.out, "\n  Episode saved to %s (%.1f MB)\n", e.OutputFile, e.SizeMB)
		} else {
			fmt.Fprintf(r.out, "\n  %s\n", e.Message)
		}
		if e.LogFile != "" {
			fmt.Fprintf(r.out, "  Log: %s  |  Total: %s\n", e.LogFile, formatElapsed(e.Elapsed))
		}
	} else if e.Stage == StageComplete {
		// Script-only or other completion without output file
		fmt.Fprintf(r.out, "\n  %s (%s)\n", e.Message, formatElapsed(e.Elapsed))
		if e.LogFile != "" {
			fmt.Fprintf(r.out, "  Log: %s\n", e.LogFile)
		}
	}
}

func (r *BarRenderer) renderTTY(e Event) {
	// Clear previous lines if any
	if r.lines > 0 {
		r.clearLines()
	}

	// Line 1: status message
	msg := fmt.Sprintf("  %s", e.Message)
	// Line 2: progress bar with percent and elapsed
	bar := renderBar(e.Percent, r.barWidth())
	pctStr := fmt.Sprintf("%3d%%", int(e.Percent*100))
	elapsed := formatElapsed(e.Elapsed)
	line2 := fmt.Sprintf("  %s %s  %s", bar, pctStr, elapsed)

	fmt.Fprintf(r.out, "%s\n%s", msg, line2)
	r.lines = 2
}

func (r *BarRenderer) renderPlain(e Event) {
	// Only print on stage transitions (not every segment update)
	fmt.Fprintf(r.out, "[%s] %s\n", formatElapsed(e.Elapsed), e.Message)
}

func (r *BarRenderer) clearLines() {
	for i := 0; i < r.lines; i++ {
		if i == 0 {
			// Clear current line and move up
			fmt.Fprint(r.out, "\r\033[2K")
		} else {
			fmt.Fprint(r.out, "\033[A\033[2K")
		}
	}
	fmt.Fprint(r.out, "\r")
	r.lines = 0
}

// barWidth returns the width available for the bar, accounting for brackets,
// percent, elapsed, and padding.
func (r *BarRenderer) barWidth() int {
	// "  [####....] 100%  0:00" → 2 + 1 + bar + 1 + 1 + 4 + 2 + 5 = bar + 16
	w := r.width - 16
	if w < 20 {
		w = 20
	}
	if w > 60 {
		w = 60
	}
	return w
}

// renderBar draws a [####....] style bar of the given width.
func renderBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return "[" + strings.Repeat("#", filled) + strings.Repeat(".", empty) + "]"
}

// formatElapsed formats a duration as M:SS.
func formatElapsed(d time.Duration) string {
	total := int(d.Seconds())
	mins := total / 60
	secs := total % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}
