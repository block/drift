package tui

import (
	"fmt"
	"strings"

	"github.com/block/drift/compare"
	"github.com/charmbracelet/x/ansi"
)

// maxDiffLines caps the number of rendered diff lines to prevent
// the viewport from choking on enormous diffs.
const maxDiffLines = 2000

// TextDiffView renders a text unified diff with colored lines.
type TextDiffView struct {
	Diff  *compare.TextDiff
	Width int
}

func (v TextDiffView) CopyableText() string {
	var b strings.Builder
	for _, h := range v.Diff.Hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, l := range h.Lines {
			switch l.Kind {
			case "added":
				b.WriteString("+ " + l.Content + "\n")
			case "removed":
				b.WriteString("- " + l.Content + "\n")
			default:
				b.WriteString("  " + l.Content + "\n")
			}
		}
	}
	return b.String()
}

func (v TextDiffView) Render() string {
	if len(v.Diff.Hunks) == 0 {
		return styleDim.Render("No text changes.")
	}

	var b strings.Builder
	lineCount := 0
	truncated := false

	for i, h := range v.Diff.Hunks {
		if truncated {
			break
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(styleHunkHeader.Render(
			fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldCount, h.NewStart, h.NewCount)))
		b.WriteString("\n")

		for _, l := range h.Lines {
			lineCount++
			if lineCount > maxDiffLines {
				truncated = true
				break
			}
			var line string
			switch l.Kind {
			case "added":
				line = styleAdded.Render(v.truncLine("+ " + l.Content))
			case "removed":
				line = styleRemoved.Render(v.truncLine("- " + l.Content))
			default:
				line = styleSubtle.Render(v.truncLine("  " + l.Content))
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	if truncated {
		total := 0
		for _, h := range v.Diff.Hunks {
			total += len(h.Lines)
		}
		b.WriteString("\n")
		b.WriteString(styleDim.Render(fmt.Sprintf("  … showing %d of %d lines (truncated)", maxDiffLines, total)))
		b.WriteString("\n")
	}

	return b.String()
}

// truncLine truncates a line to the viewport width.
func (v TextDiffView) truncLine(s string) string {
	if v.Width <= 0 {
		return s
	}
	return ansi.Truncate(s, v.Width, "…")
}
