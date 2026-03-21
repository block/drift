package tui

import (
	"fmt"
	"strings"

	"github.com/block/drift/compare"
)

// DirSummaryView renders aggregate statistics for a directory.
type DirSummaryView struct {
	Summary *compare.DirSummary
	Width   int
}

func (v DirSummaryView) Render() string {
	s := v.Summary
	var b strings.Builder

	b.WriteString(section("Contents") + "\n")
	b.WriteString(field("Files", fmt.Sprintf("%d", s.TotalFiles)))

	if s.Added > 0 || s.Removed > 0 || s.Modified > 0 {
		b.WriteString("\n")
		b.WriteString(section("Changes") + "\n")
		if s.Added > 0 {
			b.WriteString(field("Added", styleAdded.Render(fmt.Sprintf("+%d", s.Added))))
		}
		if s.Removed > 0 {
			b.WriteString(field("Removed", styleRemoved.Render(fmt.Sprintf("-%d", s.Removed))))
		}
		if s.Modified > 0 {
			b.WriteString(field("Modified", styleModified.Render(fmt.Sprintf("%d", s.Modified))))
		}
		if s.Unchanged > 0 {
			b.WriteString(field("Same", styleDim.Render(fmt.Sprintf("%d", s.Unchanged))))
		}
		if s.SizeDelta != 0 {
			b.WriteString(field("Size", styleModified.Render(formatSizeDelta(s.SizeDelta))))
		}
	}

	return b.String()
}

func (v DirSummaryView) CopyableText() string {
	s := v.Summary
	var b strings.Builder
	fmt.Fprintf(&b, "Files:    %d\n", s.TotalFiles)
	if s.Added > 0 {
		fmt.Fprintf(&b, "Added:    %d\n", s.Added)
	}
	if s.Removed > 0 {
		fmt.Fprintf(&b, "Removed:  %d\n", s.Removed)
	}
	if s.Modified > 0 {
		fmt.Fprintf(&b, "Modified: %d\n", s.Modified)
	}
	if s.Unchanged > 0 {
		fmt.Fprintf(&b, "Same:     %d\n", s.Unchanged)
	}
	if s.SizeDelta != 0 {
		fmt.Fprintf(&b, "Size:     %s\n", formatSizeDelta(s.SizeDelta))
	}
	return b.String()
}
