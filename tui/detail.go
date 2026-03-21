package tui

import (
	"github.com/block/drift/compare"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// detailModel manages the detail pane viewport.
type detailModel struct {
	viewport        viewport.Model
	node            *compare.Node
	lastDetail      *compare.DetailResult
	lastErr         error
	ready           bool
	renderedContent string // original rendered content (no highlights)
	search          string // current search query
}

func newDetailModel(width, height int) detailModel {
	vp := viewport.New(width, height)
	vp.MouseWheelEnabled = true
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
	m.viewport.Width = width
	m.viewport.Height = height
}

// SetContent sets the rendered detail content and scrolls to top.
func (m *detailModel) SetContent(node *compare.Node, detail *compare.DetailResult) {
	m.node = node
	m.lastDetail = detail
	m.lastErr = nil
	m.ready = true
	m.renderedContent = renderDetail(node, detail, m.viewport.Width)
	m.applySearch()
	m.viewport.GotoTop()
}

// SetLoading shows a loading indicator.
func (m *detailModel) SetLoading(node *compare.Node) {
	m.node = node
	m.lastDetail = nil
	m.lastErr = nil
	m.ready = true
	header := NodeHeaderView{Node: node, Width: m.viewport.Width}.Render()
	m.viewport.SetContent(header + "\n\n" + styleDim.Render("  Loading..."))
	m.viewport.GotoTop()
}

// SetError shows a structured error view.
func (m *detailModel) SetError(node *compare.Node, err error) {
	m.node = node
	m.lastDetail = nil
	m.lastErr = err
	m.ready = true
	m.viewport.SetContent(ErrorView{Node: node, Err: err, Width: m.viewport.Width}.Render())
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
		return ErrorView{Node: m.node, Err: m.lastErr, Width: m.viewport.Width}.CopyableText()
	}
	if m.lastDetail != nil {
		return copyableDetail(m.node, m.lastDetail, m.viewport.Width)
	}
	return NodeHeaderView{Node: m.node, Width: m.viewport.Width}.CopyableText()
}
