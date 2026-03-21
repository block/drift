package tui

import "github.com/block/drift/compare"

// detailView returns the appropriate View for a DetailResult.
func detailView(detail *compare.DetailResult, width int) View {
	switch {
	case detail.Dir != nil:
		return DirSummaryView{Summary: detail.Dir, Width: width}
	case detail.Text != nil:
		return TextDiffView{Diff: detail.Text, Width: width}
	case detail.Plist != nil:
		return PlistDiffView{Diff: detail.Plist, Width: width}
	case detail.Binary != nil:
		return BinaryDiffView{Diff: detail.Binary, Width: width}
	default:
		return nil
	}
}

// renderDetail composes the header and appropriate view for a DetailResult.
func renderDetail(node *compare.Node, detail *compare.DetailResult, width int) string {
	header := NodeHeaderView{Node: node, Width: width}.Render()

	if v := detailView(detail, width); v != nil {
		return header + "\n\n" + v.Render()
	}
	return header + "\n\n" + styleDim.Render("  No detailed diff available for this file type.")
}

// copyableDetail composes a plain-text representation of a detail view.
func copyableDetail(node *compare.Node, detail *compare.DetailResult, width int) string {
	header := NodeHeaderView{Node: node, Width: width}.CopyableText()

	if v := detailView(detail, width); v != nil {
		return header + "\n" + v.CopyableText()
	}
	return header
}
