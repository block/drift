package tui

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/block/drift/compare"
)

// ImageViewMode controls which image panel is displayed.
type ImageViewMode int

const (
	ImageViewSideBySide ImageViewMode = iota
	ImageViewBefore
	ImageViewAfter
	ImageViewDiff
	imageViewModeCount // sentinel for cycling
)

var imageViewModeNames = [...]string{"Side by Side", "Before", "After", "Diff"}

func (m ImageViewMode) String() string {
	if int(m) < len(imageViewModeNames) {
		return imageViewModeNames[m]
	}
	return "unknown"
}

// detailModel manages the detail pane viewport.
type detailModel struct {
	viewport        viewport.Model
	node            *compare.Node
	lastDetail      *compare.DetailResult
	lastErr         error
	ready           bool
	renderedContent string // original rendered content (no highlights)
	search          string // current search query
	imageViewMode   ImageViewMode
}

func newDetailModel(width, height int) detailModel {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	return detailModel{viewport: vp}
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m detailModel) View() string {
	if !m.ready {
		return styleDim.Render("  Select a file to view details.")
	}
	return m.viewport.View()
}

// SetSize updates the viewport dimensions.
func (m *detailModel) SetSize(width, height int) {
	m.viewport.SetWidth(width)
	m.viewport.SetHeight(height)
}

// SetContent sets the rendered detail content and scrolls to top.
func (m *detailModel) SetContent(node *compare.Node, detail *compare.DetailResult) {
	m.node = node
	m.lastDetail = detail
	m.lastErr = nil
	m.ready = true
	m.imageViewMode = ImageViewSideBySide // reset on new node
	m.renderedContent = renderDetail(node, detail, m.viewport.Width(), m.viewport.Height(), m.imageViewMode)
	m.applySearch()
	m.viewport.GotoTop()
}

// SetLoading shows a loading indicator.
func (m *detailModel) SetLoading(node *compare.Node) {
	m.node = node
	m.lastDetail = nil
	m.lastErr = nil
	m.ready = true
	header := NodeHeaderView{Node: node, Width: m.viewport.Width()}.Render()
	m.viewport.SetContent(header + "\n\n" + styleDim.Render("  Loading..."))
	m.viewport.GotoTop()
}

// SetError shows a structured error view.
func (m *detailModel) SetError(node *compare.Node, err error) {
	m.node = node
	m.lastDetail = nil
	m.lastErr = err
	m.ready = true
	m.viewport.SetContent(ErrorView{Node: node, Err: err, Width: m.viewport.Width()}.Render())
	m.viewport.GotoTop()
}

// SetSearch updates the search query and re-renders content with highlights.
func (m *detailModel) SetSearch(query string) {
	m.search = query
	if m.renderedContent != "" {
		m.applySearch()
	}
}

// SearchMatches returns the number of fuzzy-matching lines for the current query.
func (m detailModel) SearchMatches() int {
	return fuzzyCountMatches(m.renderedContent, m.search)
}

// applySearch sets viewport content with or without highlights.
func (m *detailModel) applySearch() {
	if m.search == "" {
		m.viewport.SetContent(m.renderedContent)
	} else {
		m.viewport.SetContent(fuzzyHighlightLines(m.renderedContent, m.search))
	}
}

// imageViewModeAvailable reports whether a view mode can be shown for a given diff.
func imageViewModeAvailable(mode ImageViewMode, d *compare.ImageDiff) bool {
	switch mode {
	case ImageViewSideBySide:
		return d.ImageA != nil && d.ImageB != nil
	case ImageViewBefore:
		return d.ImageA != nil
	case ImageViewAfter:
		return d.ImageB != nil
	case ImageViewDiff:
		return d.DiffMask != nil && d.PixelsChanged > 0
	default:
		return false
	}
}

// CycleImageView advances to the next available image view mode and re-renders.
// Returns true if the detail pane is showing an image (i.e., the cycle applies).
func (m *detailModel) CycleImageView() bool {
	if m.lastDetail == nil || m.lastDetail.Image == nil {
		return false
	}
	d := m.lastDetail.Image

	// Try each subsequent mode, wrapping around, until we find one that applies.
	for range int(imageViewModeCount) {
		m.imageViewMode = (m.imageViewMode + 1) % imageViewModeCount
		if imageViewModeAvailable(m.imageViewMode, d) {
			break
		}
	}

	m.rerender()
	return true
}

// IsImageView returns true if the detail pane is currently showing an image.
func (m detailModel) IsImageView() bool {
	return m.lastDetail != nil && m.lastDetail.Image != nil
}

// rerender re-renders the detail content with the current state.
func (m *detailModel) rerender() {
	if m.node == nil || m.lastDetail == nil {
		return
	}
	m.renderedContent = renderDetail(m.node, m.lastDetail, m.viewport.Width(), m.viewport.Height(), m.imageViewMode)
	m.applySearch()
	m.viewport.GotoTop()
}

// Clear resets the detail pane to its empty state.
func (m *detailModel) Clear() {
	m.node = nil
	m.lastDetail = nil
	m.lastErr = nil
	m.ready = false
	m.renderedContent = ""
	m.search = ""
}

// NodePath returns the path of the currently displayed node.
func (m detailModel) NodePath() string {
	if m.node == nil {
		return ""
	}
	return m.node.Path
}

// HasContent returns true if the detail pane has something to copy.
func (m detailModel) HasContent() bool {
	return m.ready && m.node != nil
}

// HasError returns true if the detail pane is showing an error.
func (m detailModel) HasError() bool {
	return m.lastErr != nil
}

// CopyableText returns a plain-text representation of the current detail view.
func (m detailModel) CopyableText() string {
	if m.node == nil {
		return ""
	}
	if m.lastErr != nil {
		return ErrorView{Node: m.node, Err: m.lastErr, Width: m.viewport.Width()}.CopyableText()
	}
	if m.lastDetail != nil {
		return copyableDetail(m.node, m.lastDetail, m.viewport.Width())
	}
	return NodeHeaderView{Node: m.node, Width: m.viewport.Width()}.CopyableText()
}
