package tui

import (
	"strings"

	"github.com/block/drift/compare"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View is a pure rendering component that takes props via struct fields
// and produces styled output. Views have no state or input handling.
//
// Three-tier component model:
//   - tea.Model:  stateful components with input handling (treeModel, detailModel)
//   - View:       pure render components (NodeHeaderView, TextDiffView, etc.)
//   - primitives: small building blocks (section, field, divider)
type View interface {
	// Render returns styled terminal output.
	Render() string
	// CopyableText returns a plain-text representation for clipboard export.
	CopyableText() string
}

// section renders a titled section heading.
//
//	section("Symbols")  →  "Symbols\n"
func section(title string) string {
	return styleTitle.Render(title) + "\n"
}

// field renders a labeled key-value field with aligned columns.
//
//	field("Status", "● modified")  →  "Status  ● modified\n"
func field(label, value string) string {
	// Pad label to 8 chars for alignment.
	padded := label
	if len(padded) < 8 {
		padded += strings.Repeat(" ", 8-len(padded))
	}
	return styleSubtle.Render(padded) + value + "\n"
}

// divider renders a horizontal rule.
func divider(width int) string {
	return styleDim.Render(strings.Repeat("─", min(width, 50)))
}

// statusLabel renders a colored status indicator with text.
func statusLabel(s compare.DiffStatus) string {
	switch s {
	case compare.Added:
		return styleAdded.Render("● added")
	case compare.Removed:
		return styleRemoved.Render("● removed")
	case compare.Modified:
		return styleModified.Render("● modified")
	default:
		return styleDim.Render("● unchanged")
	}
}

// paneHeader renders a pane title with a divider line and optional badge on a second line.
func paneHeader(title string, focused bool, width int, badge ...string) string {
	style := stylePaneTitle
	if focused {
		style = stylePaneTitleFocused
	}
	label := style.Render(title)
	fillW := max(0, width-lipgloss.Width(label))
	header := label + styleDim.Render(strings.Repeat("─", fillW))

	if len(badge) > 0 && badge[0] != "" {
		header += "\n" + badge[0]
	}
	return header
}

// wrapText wraps a string to fit within maxWidth visual columns.
func wrapText(s string, maxWidth int) string {
	if maxWidth <= 0 || ansi.StringWidth(s) <= maxWidth {
		return s
	}
	var lines []string
	for ansi.StringWidth(s) > maxWidth {
		lines = append(lines, ansi.Truncate(s, maxWidth, ""))
		s = ansi.TruncateLeft(s, maxWidth, "")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return strings.Join(lines, "\n")
}
