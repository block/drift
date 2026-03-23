package tui

import "github.com/block/drift/compare"

// detailView returns the appropriate View for a DetailResult.
func detailView(detail *compare.DetailResult, git *compare.GitMeta, width, height int, imgMode ImageViewMode) View {
	switch {
	case detail.Dir != nil:
		if git != nil {
			return GitMetaView{Git: git, Dir: detail.Dir, Width: width}
		}
		return DirSummaryView{Summary: detail.Dir, Width: width}
	case detail.Text != nil:
		return TextDiffView{Diff: detail.Text, Width: width}
	case detail.Plist != nil:
		return PlistDiffView{Diff: detail.Plist, Width: width}
	case detail.Binary != nil:
		return BinaryDiffView{Diff: detail.Binary, Width: width}
	case detail.Image != nil:
		return ImageDiffView{Diff: detail.Image, Width: width, Height: height, Mode: imgMode}
	default:
		return nil
	}
}

// renderDetail composes the header and appropriate view for a DetailResult.
func renderDetail(node *compare.Node, detail *compare.DetailResult, git *compare.GitMeta, width, height int, imgMode ImageViewMode) string {
	v := detailView(detail, git, width, height, imgMode)

	// GitMetaView renders its own header - skip the default NodeHeaderView.
	if _, isGit := v.(GitMetaView); isGit {
		return v.Render()
	}

	header := NodeHeaderView{Node: node, Width: width}.Render()
	if v != nil {
		return header + "\n\n" + v.Render()
	}
	return header + "\n\n" + styleDim.Render("  No detailed diff available for this file type.")
}

// copyableDetail composes a plain-text representation of a detail view.
func copyableDetail(node *compare.Node, detail *compare.DetailResult, git *compare.GitMeta, width int) string {
	v := detailView(detail, git, width, 0, ImageViewSideBySide)

	// GitMetaView renders its own header.
	if _, isGit := v.(GitMetaView); isGit {
		return v.CopyableText()
	}

	header := NodeHeaderView{Node: node, Width: width}.CopyableText()
	if v != nil {
		return header + "\n" + v.CopyableText()
	}
	return header
}
