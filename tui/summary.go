package tui

import (
	"fmt"

	"github.com/block/drift/compare"
)

// SummaryBarView renders the aggregate diff statistics bar.
type SummaryBarView struct {
	Summary compare.Summary
	Width   int
}

func (v SummaryBarView) Render() string {
	var parts []string

	if v.Summary.Added > 0 {
		parts = append(parts, styleAdded.Render(fmt.Sprintf("+%d added", v.Summary.Added)))
	}
	if v.Summary.Removed > 0 {
		parts = append(parts, styleRemoved.Render(fmt.Sprintf("-%d removed", v.Summary.Removed)))
	}
	if v.Summary.Modified > 0 {
		parts = append(parts, styleModified.Render(fmt.Sprintf("~%d modified", v.Summary.Modified)))
	}
	if v.Summary.Unchanged > 0 {
		parts = append(parts, styleDim.Render(fmt.Sprintf("%d unchanged", v.Summary.Unchanged)))
	}

	line := ""
	for i, p := range parts {
		if i > 0 {
			line += styleDim.Render(" · ")
		}
		line += p
	}

	if v.Summary.SizeDelta != 0 {
		line += styleDim.Render("  ") + styleModified.Render(formatSizeDelta(v.Summary.SizeDelta))
	}

	return styleSummaryBar.Width(v.Width).Render(line)
}

// formatSize formats a byte count as a human-readable string.
func formatSize(b int64) string {
	abs := b
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case abs >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case abs >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatSizeDelta formats a size delta with a +/- prefix.
func formatSizeDelta(b int64) string {
	s := formatSize(b)
	if b > 0 {
		return "+" + s
	}
	return s
}
