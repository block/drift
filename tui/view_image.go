package tui

import (
	"fmt"
	"image"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/block/drift/compare"
)

// ImageDiffView renders an image comparison with metadata and half-block thumbnails.
type ImageDiffView struct {
	Diff   *compare.ImageDiff
	Width  int
	Height int // available viewport rows for the entire view
	Mode   ImageViewMode
}

// fieldAB renders a labeled field for an A/B comparison. When both values are
// present and differ, they're shown with sep between them. If either value is
// empty, only the non-empty one is shown. Returns "" if both are empty.
func fieldAB(label, a, b, sep string, render func(label, value string) string) string {
	switch {
	case a != "" && b != "":
		if a == b {
			return render(label, a)
		}
		return render(label, a+sep+b)
	case a != "":
		return render(label, a)
	case b != "":
		return render(label, b)
	default:
		return ""
	}
}

func (v ImageDiffView) CopyableText() string {
	var b strings.Builder
	d := v.Diff

	plainField := func(label, value string) string {
		return fmt.Sprintf("%-8s %s\n", label, value)
	}

	b.WriteString("Image Comparison\n")
	b.WriteString(fieldAB("Format", d.FormatA, d.FormatB, " → ", plainField))

	dimA, dimB := "", ""
	if d.ImageA != nil {
		dimA = fmt.Sprintf("%dx%d", d.WidthA, d.HeightA)
	}
	if d.ImageB != nil {
		dimB = fmt.Sprintf("%dx%d", d.WidthB, d.HeightB)
	}
	b.WriteString(fieldAB("Size", dimA, dimB, " → ", plainField))

	if d.PixelsTotal > 0 {
		fmt.Fprintf(&b, "Changed: %d / %d px (%.1f%%)\n", d.PixelsChanged, d.PixelsTotal, d.ChangePercent)
		if d.PixelsChanged > 0 {
			r := d.ChangeBounds
			fmt.Fprintf(&b, "Region:  (%d,%d) → (%d,%d)\n", r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
		}
	}

	return b.String()
}

func (v ImageDiffView) Render() string {
	d := v.Diff
	var b strings.Builder

	// Metadata section.
	b.WriteString(v.renderMetadata())

	hasA := d.ImageA != nil
	hasB := d.ImageB != nil
	contentWidth := v.Width - panePadding

	// For added/removed, there's only one image - ignore Mode.
	if hasA && !hasB {
		b.WriteString("\n")
		b.WriteString(v.renderFullImage(d.ImageA, "Image", contentWidth))
		return b.String()
	}
	if !hasA && hasB {
		b.WriteString("\n")
		b.WriteString(v.renderFullImage(d.ImageB, "Image", contentWidth))
		return b.String()
	}

	if !hasA && !hasB {
		return b.String()
	}

	// Both images present - render based on current mode.
	b.WriteString("\n")

	// Mode indicator.
	b.WriteString("  " + styleDim.Render("View: ") + styleAccent.Render(v.Mode.String()))
	b.WriteString(styleDim.Render("  (v to cycle)") + "\n\n")

	switch v.Mode {
	case ImageViewSideBySide:
		b.WriteString(v.renderSideBySide(contentWidth))
	case ImageViewBefore:
		b.WriteString(v.renderFullImage(d.ImageA, "Before", contentWidth))
	case ImageViewAfter:
		b.WriteString(v.renderFullImage(d.ImageB, "After", contentWidth))
	case ImageViewDiff:
		if d.DiffMask != nil && d.PixelsChanged > 0 {
			b.WriteString(v.renderDiffPanel(contentWidth))
		} else {
			b.WriteString(styleDim.Render("  No pixel differences to show."))
		}
	}

	return b.String()
}

// renderMetadata renders the image metadata fields.
func (v ImageDiffView) renderMetadata() string {
	d := v.Diff
	var b strings.Builder

	b.WriteString(section("Image Comparison"))
	b.WriteString("\n")

	styledArrow := styleDim.Render(" → ")
	b.WriteString(fieldAB("Format", d.FormatA, d.FormatB, styledArrow, field))

	dimA, dimB := "", ""
	if d.ImageA != nil {
		dimA = fmt.Sprintf("%dx%d", d.WidthA, d.HeightA)
	}
	if d.ImageB != nil {
		dimB = fmt.Sprintf("%dx%d", d.WidthB, d.HeightB)
	}
	b.WriteString(fieldAB("Size", dimA, dimB, styledArrow, field))

	b.WriteString(fieldAB("Color", d.ColorModelA, d.ColorModelB, styledArrow, field))

	// Pixel diff stats.
	if d.PixelsTotal > 0 {
		if d.PixelsChanged == 0 {
			b.WriteString(field("Pixels", styleDim.Render("identical")))
		} else {
			changed := fmt.Sprintf("%d / %d", d.PixelsChanged, d.PixelsTotal)
			pct := fmt.Sprintf(" (%.1f%%)", d.ChangePercent)
			b.WriteString(field("Changed", styleModified.Render(changed)+styleDim.Render(pct)))

			r := d.ChangeBounds
			region := fmt.Sprintf("(%d,%d) → (%d,%d)", r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
			b.WriteString(field("Region", region))
		}
	} else if d.ImageA != nil && d.ImageB != nil {
		b.WriteString(field("Pixels", styleDim.Render("dimensions differ, pixel comparison skipped")))
	}

	return b.String()
}

// imageRows returns the max terminal rows available for image rendering,
// accounting for metadata chrome. Returns 0 if unconstrained.
func (v ImageDiffView) imageRows() int {
	if v.Height <= 0 {
		return 0
	}
	// Chrome breakdown: node header (3) + section title (2) + metadata fields (4)
	// + mode indicator (2) + image label (1) + spacing (2) = 14 lines.
	const metadataChrome = 14
	const minImageRows = 4
	rows := v.Height - metadataChrome
	if rows < minImageRows {
		rows = minImageRows
	}
	return rows
}

// renderSideBySide renders before/after thumbnails side by side.
func (v ImageDiffView) renderSideBySide(contentWidth int) string {
	d := v.Diff

	gap := 4
	thumbWidth := (contentWidth - gap) / 2
	if thumbWidth < 10 {
		thumbWidth = 10
	}

	maxRows := v.imageRows()
	beforeStr := renderHalfBlock(d.ImageA, thumbWidth, maxRows)
	afterStr := renderHalfBlock(d.ImageB, thumbWidth, maxRows)

	beforeLines := strings.Split(beforeStr, "\n")
	afterLines := strings.Split(afterStr, "\n")

	maxLines := max(len(beforeLines), len(afterLines))
	for len(beforeLines) < maxLines {
		beforeLines = append(beforeLines, "")
	}
	for len(afterLines) < maxLines {
		afterLines = append(afterLines, "")
	}

	var b strings.Builder

	// Labels.
	beforeLabel := stylePaneTitle.Render("Before")
	afterLabel := stylePaneTitle.Render("After")
	labelPad := strings.Repeat(" ", max(0, thumbWidth-lipgloss.Width(beforeLabel)+gap))
	b.WriteString("  " + beforeLabel + labelPad + afterLabel + "\n")

	// Image rows.
	spacer := strings.Repeat(" ", gap)
	for i := range maxLines {
		left := beforeLines[i]
		right := afterLines[i]
		leftW := lipgloss.Width(left)
		pad := strings.Repeat(" ", max(0, thumbWidth-leftW))
		b.WriteString("  " + left + pad + spacer + right + "\n")
	}

	return b.String()
}

// renderImagePanel renders a labeled image block with consistent indentation.
func renderImagePanel(label, imgStr string) string {
	var b strings.Builder
	b.WriteString("  " + stylePaneTitle.Render(label) + "\n")
	for _, line := range strings.Split(imgStr, "\n") {
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

// renderDiffPanel renders the diff mask overlay on the "after" image at full width.
func (v ImageDiffView) renderDiffPanel(contentWidth int) string {
	d := v.Diff
	diffStr := renderHalfBlockOverlay(d.ImageB, d.DiffMask, contentWidth, v.imageRows())
	return renderImagePanel("Diff", diffStr)
}

// renderFullImage renders a single image at full available width.
func (v ImageDiffView) renderFullImage(img image.Image, label string, contentWidth int) string {
	imgStr := renderHalfBlock(img, contentWidth, v.imageRows())
	return renderImagePanel(label, imgStr)
}
