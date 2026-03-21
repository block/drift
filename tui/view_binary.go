package tui

import (
	"fmt"
	"strings"

	"github.com/block/drift/compare"
	"github.com/charmbracelet/x/ansi"
)

// maxSymbols caps the number of rendered symbols to prevent
// the viewport from choking on binaries with massive symbol tables.
const maxSymbols = 500

// BinaryDiffView renders symbol and section changes with a summary.
type BinaryDiffView struct {
	Diff  *compare.BinaryDiff
	Width int
}

func (v BinaryDiffView) CopyableText() string {
	var b strings.Builder
	if len(v.Diff.Symbols) > 0 {
		b.WriteString("Symbols:\n")
		for _, s := range v.Diff.Symbols {
			fmt.Fprintf(&b, "  %s %s\n", s.Status, s.Name)
		}
	}
	if len(v.Diff.Sections) > 0 {
		if len(v.Diff.Symbols) > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Sections:\n")
		for _, s := range v.Diff.Sections {
			fmt.Fprintf(&b, "  %s.%s  %s → %s  (%s)\n",
				s.Segment, s.Section,
				formatSize(s.SizeA), formatSize(s.SizeB),
				formatSizeDelta(s.SizeB-s.SizeA))
		}
	}
	return b.String()
}

func (v BinaryDiffView) Render() string {
	if len(v.Diff.Symbols) == 0 && len(v.Diff.Sections) == 0 {
		return styleDim.Render("No binary changes detected.")
	}

	var b strings.Builder

	// Summary counts.
	var symAdded, symRemoved int
	for _, s := range v.Diff.Symbols {
		switch s.Status {
		case compare.Added:
			symAdded++
		case compare.Removed:
			symRemoved++
		}
	}
	var totalSizeDelta int64
	for _, s := range v.Diff.Sections {
		totalSizeDelta += s.SizeB - s.SizeA
	}

	b.WriteString(section("Summary") + "\n")
	if len(v.Diff.Symbols) > 0 {
		var parts []string
		if symAdded > 0 {
			parts = append(parts, styleAdded.Render(fmt.Sprintf("+%d added", symAdded)))
		}
		if symRemoved > 0 {
			parts = append(parts, styleRemoved.Render(fmt.Sprintf("-%d removed", symRemoved)))
		}
		b.WriteString(styleSubtle.Render("  Symbols    "))
		for i, p := range parts {
			if i > 0 {
				b.WriteString(styleDim.Render(", "))
			}
			b.WriteString(p)
		}
		b.WriteString("\n")
	}
	if len(v.Diff.Sections) > 0 {
		b.WriteString(styleSubtle.Render("  Sections   "))
		fmt.Fprintf(&b, "%d changed", len(v.Diff.Sections))
		if totalSizeDelta != 0 {
			b.WriteString("  " + styleModified.Render(formatSizeDelta(totalSizeDelta)))
		}
		b.WriteString("\n")
	}

	// Symbol details.
	if len(v.Diff.Symbols) > 0 {
		b.WriteString("\n")
		b.WriteString(section("Symbols") + "\n")

		displayCount := min(len(v.Diff.Symbols), maxSymbols)
		for _, s := range v.Diff.Symbols[:displayCount] {
			icon := statusIcon(s.Status, false, false)
			name := v.truncLine(s.Name)
			b.WriteString("  " + icon + " " + styleChangeKey.Render(name) + "\n")
		}
		if len(v.Diff.Symbols) > maxSymbols {
			b.WriteString("\n")
			b.WriteString(styleDim.Render(fmt.Sprintf(
				"  … showing %d of %d symbols (truncated)", maxSymbols, len(v.Diff.Symbols))))
			b.WriteString("\n")
		}
	}

	// Section details.
	if len(v.Diff.Sections) > 0 {
		b.WriteString("\n")
		b.WriteString(section("Sections") + "\n")
		for _, s := range v.Diff.Sections {
			name := styleSectionName.Render(s.Segment + "." + s.Section)
			fmt.Fprintf(&b, "  %s\n    %s → %s  %s\n",
				name,
				styleSubtle.Render(formatSize(s.SizeA)),
				styleSubtle.Render(formatSize(s.SizeB)),
				styleModified.Render(formatSizeDelta(s.SizeB-s.SizeA)))
		}
	}

	return b.String()
}

// truncLine truncates a line to the viewport width.
func (v BinaryDiffView) truncLine(s string) string {
	if v.Width <= 0 {
		return s
	}
	// Leave room for icon prefix ("  + ").
	maxW := v.Width - 4
	if maxW <= 0 {
		return s
	}
	return ansi.Truncate(s, maxW, "…")
}
