package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// placeOverlay composites a foreground string on top of a background string
// at the given x, y position. Both strings may contain ANSI escape sequences.
func placeOverlay(x, y int, fg, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")

	for i, fgLine := range fgLines {
		bgIdx := y + i
		if bgIdx < 0 || bgIdx >= len(bgLines) {
			continue
		}

		bgLine := bgLines[bgIdx]
		bgWidth := ansi.StringWidth(bgLine)
		fgWidth := ansi.StringWidth(fgLine)

		if x >= bgWidth {
			// Foreground is past the background - pad and append.
			bgLines[bgIdx] = bgLine + strings.Repeat(" ", x-bgWidth) + fgLine
			continue
		}

		// Composite: left of bg + fg + right of bg.
		left := ansi.Truncate(bgLine, x, "")
		right := ""
		if x+fgWidth < bgWidth {
			right = ansi.TruncateLeft(bgLine, x+fgWidth, "")
		}

		bgLines[bgIdx] = left + fgLine + right
	}

	return strings.Join(bgLines, "\n")
}

// centerOverlay places a foreground string centered on a background string.
func centerOverlay(fg, bg string, bgWidth, bgHeight int) string {
	fgLines := strings.Split(fg, "\n")
	fgWidth := 0
	for _, l := range fgLines {
		if w := lipgloss.Width(l); w > fgWidth {
			fgWidth = w
		}
	}
	fgHeight := len(fgLines)

	x := (bgWidth - fgWidth) / 2
	y := (bgHeight - fgHeight) / 2

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return placeOverlay(x, y, fg, bg)
}
