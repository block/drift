package tui

import (
	"fmt"
	"strings"

	"github.com/block/drift/compare"
	"github.com/charmbracelet/x/ansi"
)

// maxPlistChanges caps the number of rendered plist changes.
const maxPlistChanges = 1000

// PlistDiffView renders plist key-path changes.
type PlistDiffView struct {
	Diff  *compare.PlistDiff
	Width int
}

func (v PlistDiffView) CopyableText() string {
	var b strings.Builder
	for _, c := range v.Diff.Changes {
		fmt.Fprintf(&b, "%s %s\n", c.Status, c.KeyPath)
		switch c.Status {
		case compare.Modified:
			b.WriteString("  - " + c.ValueA + "\n")
			b.WriteString("  + " + c.ValueB + "\n")
		case compare.Added:
			b.WriteString("  + " + c.ValueB + "\n")
		case compare.Removed:
			b.WriteString("  - " + c.ValueA + "\n")
		}
	}
	return b.String()
}

func (v PlistDiffView) Render() string {
	if len(v.Diff.Changes) == 0 {
		return styleDim.Render("No plist changes.")
	}

	displayCount := min(len(v.Diff.Changes), maxPlistChanges)

	var b strings.Builder
	for i, c := range v.Diff.Changes[:displayCount] {
		if i > 0 {
			b.WriteString("\n")
		}
		icon := statusIcon(c.Status, false, false)
		b.WriteString(icon + " " + styleChangeKeyBold.Render(v.truncLine(c.KeyPath)) + "\n")

		switch c.Status {
		case compare.Modified:
			b.WriteString("  " + styleRemoved.Render(v.truncLine("- "+c.ValueA)) + "\n")
			b.WriteString("  " + styleAdded.Render(v.truncLine("+ "+c.ValueB)) + "\n")
		case compare.Added:
			b.WriteString("  " + styleAdded.Render(v.truncLine("+ "+c.ValueB)) + "\n")
		case compare.Removed:
			b.WriteString("  " + styleRemoved.Render(v.truncLine("- "+c.ValueA)) + "\n")
		}
	}

	if len(v.Diff.Changes) > maxPlistChanges {
		b.WriteString("\n")
		b.WriteString(styleDim.Render(fmt.Sprintf(
			"  … showing %d of %d changes (truncated)", maxPlistChanges, len(v.Diff.Changes))))
		b.WriteString("\n")
	}

	return b.String()
}

// truncLine truncates a line to the viewport width.
func (v PlistDiffView) truncLine(s string) string {
	if v.Width <= 0 {
		return s
	}
	return ansi.Truncate(s, v.Width, "…")
}
