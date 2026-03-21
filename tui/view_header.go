package tui

import (
	"fmt"
	"strings"

	"github.com/block/drift/compare"
)

// NodeHeaderView renders the file info header used at the top of all detail views.
type NodeHeaderView struct {
	Node  *compare.Node
	Width int
}

func (v NodeHeaderView) Render() string {
	var b strings.Builder

	name := styleTitle.Render(v.Node.Name)
	if badge := kindBadge(v.Node.Kind); badge != "" {
		name += " " + styleKindBadge.Render(badge)
	}
	b.WriteString(name)
	b.WriteString("\n\n")

	b.WriteString(field("Status", statusLabel(v.Node.Status)))

	if v.Node.SizeA > 0 || v.Node.SizeB > 0 {
		sizeValue := styleSubtle.Render(formatSize(v.Node.SizeA)) +
			styleSubtle.Render(" → ") +
			styleSubtle.Render(formatSize(v.Node.SizeB))
		if delta := v.Node.SizeDelta(); delta != 0 {
			sizeValue += "  " + styleModified.Render(formatSizeDelta(delta))
		}
		b.WriteString(field("Size", sizeValue))
	}

	b.WriteString(divider(v.Width))
	return b.String()
}

func (v NodeHeaderView) CopyableText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "File:   %s\n", v.Node.Name)
	fmt.Fprintf(&b, "Path:   %s\n", v.Node.Path)
	fmt.Fprintf(&b, "Kind:   %s\n", v.Node.Kind)
	fmt.Fprintf(&b, "Status: %s\n", v.Node.Status)

	if v.Node.SizeA > 0 || v.Node.SizeB > 0 {
		fmt.Fprintf(&b, "Size:   %s → %s", formatSize(v.Node.SizeA), formatSize(v.Node.SizeB))
		if delta := v.Node.SizeDelta(); delta != 0 {
			fmt.Fprintf(&b, " (%s)", formatSizeDelta(delta))
		}
		b.WriteString("\n")
	}

	return b.String()
}
