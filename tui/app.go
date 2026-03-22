package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"
	"github.com/block/drift/compare"
)

// focusedPane tracks which pane has keyboard focus.
type focusedPane int

const (
	paneTree focusedPane = iota
	paneDetail
)

// detailLoadedMsg delivers a completed Detail() result.
type detailLoadedMsg struct {
	node   *compare.Node
	detail *compare.DetailResult
	err    error
}

// clipboardMsg signals that a clipboard copy completed.
type clipboardMsg struct{ err error }

// swapCompleteMsg delivers the result of a swap operation.
type swapCompleteMsg struct {
	result *compare.Result
	err    error
}

// Model is the root TUI model.
type Model struct {
	result     *compare.Result
	keys       keyMap
	help       help.Model
	tree       treeModel
	detail     detailModel
	alert      alertModel
	search     searchModel
	focus      focusedPane
	width      int
	height     int
	standalone bool // true for non-tree modes (text, plist, binary)
}

// New creates a new TUI model from a comparison result.
func New(result *compare.Result) Model {
	keys := newKeyMap()
	h := help.New()
	tree := newTreeModel(result.Root, keys, 80, 24)
	detail := newDetailModel(40, 24)

	standalone := result.Mode != "tree"
	focus := paneTree
	if standalone {
		focus = paneDetail
		// Disable tree-only keybindings in standalone mode.
		keys.ToggleFocus.SetEnabled(false)
		keys.Expand.SetEnabled(false)
		keys.Collapse.SetEnabled(false)
		keys.NextChange.SetEnabled(false)
		keys.PrevChange.SetEnabled(false)
		keys.Filter.SetEnabled(false)
		keys.FilterAll.SetEnabled(false)
		keys.FilterAdded.SetEnabled(false)
		keys.FilterRemoved.SetEnabled(false)
		keys.FilterModified.SetEnabled(false)
	}

	return Model{
		result:     result,
		keys:       keys,
		help:       h,
		tree:       tree,
		detail:     detail,
		search:     newSearchModel(),
		focus:      focus,
		standalone: standalone,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestWindowSize}
	if m.standalone {
		// Auto-load detail for the single root node.
		cmds = append(cmds, func() tea.Msg {
			return nodeSelectedMsg{node: m.result.Root}
		})
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.SetWidth(msg.Width)
		m.layout()
		return m, nil

	case tea.KeyPressMsg:
		// While search is active, route input to the search bar.
		if m.search.Active() {
			return m.updateSearch(msg)
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
			m.layout()
			return m, nil
		case key.Matches(msg, m.keys.ToggleFocus):
			if m.focus == paneTree {
				m.focus = paneDetail
			} else {
				m.focus = paneTree
			}
			return m, nil
		case key.Matches(msg, m.keys.Copy):
			if m.detail.HasContent() {
				return m, copyToClipboard(m.detail.CopyableText())
			}
		case key.Matches(msg, m.keys.ImageView):
			if m.detail.CycleImageView() {
				return m, nil
			}
		case key.Matches(msg, m.keys.Swap):
			return m, m.swapPaths()
		case key.Matches(msg, m.keys.Search):
			cmd := m.search.Activate()
			m.layout()
			return m, cmd
		case msg.Code == tea.KeyEscape && m.search.HasQuery():
			// Clear confirmed search.
			m.search.Deactivate()
			m.applySearchQuery("")
			m.layout()
			return m, nil
		}

	case tea.MouseClickMsg:
		if !m.standalone && msg.Button == tea.MouseLeft {
			treeWidth := int(float64(m.width) * treeSplitRatio)
			if msg.X < treeWidth {
				m.focus = paneTree
				// Map click Y to tree item row.
				row := msg.Y - m.treeContentY()
				if row >= 0 {
					var cmd tea.Cmd
					m.tree, cmd = m.tree.HandleClick(row)
					if cmd == nil {
						cmd = m.autoLoadDetail()
					}
					return m, cmd
				}
			} else {
				m.focus = paneDetail
			}
		}

	case swapCompleteMsg:
		if msg.err != nil {
			return m, m.alert.Show("Swap failed: "+msg.err.Error(), alertError)
		}
		m.result = msg.result
		m.tree = newTreeModel(msg.result.Root, m.keys, m.width, m.height)
		m.detail.Clear()
		m.keys.Copy.SetEnabled(false)
		m.layout()
		if m.standalone {
			return m, func() tea.Msg {
				return nodeSelectedMsg{node: msg.result.Root}
			}
		}
		return m, m.alert.Show("Swapped A ↔ B", alertInfo)

	case nodeSelectedMsg:
		// Load detail if not already showing this node.
		if msg.node.Path != m.detail.NodePath() {
			return m, m.loadDetailFor(msg.node)
		}
		return m, nil

	case detailLoadedMsg:
		// Discard stale results if the user navigated away.
		if msg.node.Path != m.detail.NodePath() {
			return m, nil
		}
		if msg.err != nil {
			m.detail.SetError(msg.node, msg.err)
		} else {
			m.detail.SetContent(msg.node, msg.detail)
		}
		m.keys.Copy.SetEnabled(true)
		m.keys.ImageView.SetEnabled(m.detail.IsImageView())
		return m, nil

	case clipboardMsg:
		if msg.err != nil {
			return m, m.alert.Show("Failed to copy: "+msg.err.Error(), alertError)
		}
		return m, m.alert.Show("Copied to clipboard", alertSuccess)

	case filterChangedMsg:
		m.detail.Clear()
		m.keys.Copy.SetEnabled(false)
		m.keys.ImageView.SetEnabled(false)
		// Auto-load detail for whatever node is now selected.
		if cmd := m.autoLoadDetail(); cmd != nil {
			return m, cmd
		}
		return m, nil

	case alertDismissMsg:
		m.alert.Dismiss()
		return m, nil
	}

	// Route messages to the focused pane.
	var cmd tea.Cmd
	switch m.focus {
	case paneTree:
		m.tree, cmd = m.tree.Update(msg)
		// Auto-load detail on cursor change, but not when tree emitted its own cmd
		// (e.g., filterChangedMsg - that handler does its own load).
		if cmd == nil {
			if autoCmd := m.autoLoadDetail(); autoCmd != nil {
				cmd = autoCmd
			}
		}
	case paneDetail:
		m.detail, cmd = m.detail.Update(msg)
	}

	return m, cmd
}

// updateSearch handles key messages while the search bar is active.
func (m Model) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEscape:
		// Cancel: clear search and restore.
		m.search.Deactivate()
		m.applySearchQuery("")
		m.layout()
		return m, nil
	case tea.KeyEnter:
		// Confirm: keep filter, exit search input.
		m.search.Confirm()
		m.layout()
		return m, nil
	}

	// Forward to text input.
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(msg)

	// Live-update filter as user types.
	m.applySearchQuery(m.search.Query())

	return m, cmd
}

// applySearchQuery updates the tree or detail with the current search query.
func (m *Model) applySearchQuery(query string) {
	switch m.focus {
	case paneTree:
		m.tree.SetSearch(query)
		m.search.SetMatches(m.tree.SearchMatches())
	case paneDetail:
		m.detail.SetSearch(query)
		m.search.SetMatches(m.detail.SearchMatches())
	}
}

// autoLoadDetail checks if the tree cursor moved to a new node and loads its detail.
func (m *Model) autoLoadDetail() tea.Cmd {
	node := m.tree.SelectedNode()
	if node == nil {
		return nil
	}
	if node.Path == m.detail.NodePath() {
		return nil
	}
	return m.loadDetailFor(node)
}

// copyToClipboard copies text to the system clipboard.
func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(text)
		return clipboardMsg{err: err}
	}
}

// swapPaths re-runs comparison with A and B swapped.
func (m Model) swapPaths() tea.Cmd {
	pathA, pathB, mode := m.result.PathA, m.result.PathB, m.result.Mode
	return func() tea.Msg {
		result, err := compare.Compare(pathB, pathA, mode)
		return swapCompleteMsg{result: result, err: err}
	}
}

// loadDetailFor sets loading state and returns a cmd to fetch detail.
func (m *Model) loadDetailFor(node *compare.Node) tea.Cmd {
	m.detail.SetLoading(node)
	result := m.result
	return func() tea.Msg {
		detail, err := compare.Detail(result, node)
		return detailLoadedMsg{node: node, detail: detail, err: err}
	}
}

func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.View{AltScreen: true, MouseMode: tea.MouseModeCellMotion}
	}

	header := m.renderHeader()
	summary := SummaryBarView{Summary: m.result.Summary, Width: m.width}.Render()
	helpView := styleHelpBar.Width(m.width).Render(m.help.View(m.keys))
	footerDiv := styleDim.Render(strings.Repeat("─", m.width))

	chrome := lipgloss.Height(header) + 1 + lipgloss.Height(summary) + 1 + lipgloss.Height(helpView)
	contentHeight := m.height - chrome
	if contentHeight < 1 {
		return tea.View{
			Content:   header + "\n" + summary + "\n" + helpView,
			AltScreen: true,
			MouseMode: tea.MouseModeCellMotion,
		}
	}

	var content string

	if m.standalone {
		// Full-width detail for standalone mode.
		// In lipgloss v2, Width/Height include borders, so pass total dimensions.
		detailInnerW := m.width - panePadding
		detailStyle := styleFocusedBorder.Width(m.width).Height(contentHeight).MaxHeight(contentHeight)

		content = detailStyle.Render(m.detailPaneChrome(detailInnerW) + m.detail.View())
	} else {
		// Split layout: tree | detail.
		treeWidth := int(float64(m.width) * treeSplitRatio)
		detailWidth := m.width - treeWidth

		// Tree pane with filter badge and optional search bar.
		// In lipgloss v2, Width/Height include borders, so pass total pane dimensions.
		treeInnerW := treeWidth - panePadding
		treeStyle := m.paneStyle(paneTree).Width(treeWidth).Height(contentHeight).MaxHeight(contentHeight)
		treeView := treeStyle.Render(m.treePaneChrome(treeInnerW) + m.tree.View())

		// Detail pane with optional search bar.
		detailInnerW := detailWidth - panePadding
		detailStyle := m.paneStyle(paneDetail).Width(detailWidth).Height(contentHeight).MaxHeight(contentHeight)
		detailView := detailStyle.Render(m.detailPaneChrome(detailInnerW) + m.detail.View())

		content = lipgloss.JoinHorizontal(lipgloss.Top, treeView, detailView)
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		header,
		content,
		footerDiv,
		summary,
		"",
		helpView,
	)

	if m.alert.Visible() {
		view = centerOverlay(m.alert.Render(), view, m.width, m.height)
	}

	return tea.View{
		Content:   view,
		AltScreen: true,
		MouseMode: tea.MouseModeCellMotion,
	}
}

// searchActiveFor returns true if the search bar should be shown in the given pane.
func (m Model) searchActiveFor(pane focusedPane) bool {
	return m.focus == pane && (m.search.Active() || m.search.HasQuery())
}

func (m Model) renderHeader() string {
	title := styleHeaderLabel.Render("drift: ") +
		styleTitle.Render(m.result.Root.Name)
	return styleSummaryBar.Width(m.width).Render(title)
}

func (m Model) filterBadge() string {
	f := m.tree.Filter()
	switch f {
	case filterAdded:
		return styleAdded.Render("+ " + f.String())
	case filterRemoved:
		return styleRemoved.Render("− " + f.String())
	case filterModified:
		return styleModified.Render("● " + f.String())
	default:
		return styleSubtle.Render("◇ " + f.String())
	}
}

// treeContentY returns the absolute Y coordinate where tree items start.
func (m Model) treeContentY() int {
	headerH := lipgloss.Height(m.renderHeader())
	treeWidth := int(float64(m.width) * treeSplitRatio)
	treeInnerW := treeWidth - panePadding

	// Build the same chrome that View() renders above the tree content.
	chrome := m.treePaneChrome(treeInnerW)

	// +1 for the pane's top border line.
	return headerH + 1 + lipgloss.Height(chrome)
}

// treePaneChrome returns the content rendered above the tree list inside the pane
// (pane header, filter badge, optional search bar, and trailing newline).
func (m Model) treePaneChrome(innerW int) string {
	chrome := paneHeader("Files", m.focus == paneTree, innerW, m.filterBadge())
	if m.searchActiveFor(paneTree) {
		chrome += "\n" + m.search.View(innerW)
	}
	chrome += "\n" // the separator before tree.View()
	return chrome
}

// detailPaneChrome returns the content rendered above the detail viewport inside the pane.
func (m Model) detailPaneChrome(innerW int) string {
	chrome := paneHeader("Details", m.focus == paneDetail, innerW)
	if m.searchActiveFor(paneDetail) {
		chrome += "\n" + m.search.View(innerW)
	}
	chrome += "\n" // the separator before detail.View()
	return chrome
}

func (m Model) paneStyle(pane focusedPane) lipgloss.Style {
	if m.focus == pane {
		return styleFocusedBorder
	}
	return styleBlurredBorder
}

// layout recalculates component sizes after a resize or help toggle.
func (m *Model) layout() {
	header := m.renderHeader()
	summary := SummaryBarView{Summary: m.result.Summary, Width: m.width}.Render()
	helpView := styleHelpBar.Width(m.width).Render(m.help.View(m.keys))

	appChrome := lipgloss.Height(header) + 1 + lipgloss.Height(summary) + 1 + lipgloss.Height(helpView)
	innerH := m.height - appChrome - borderHeight

	if m.standalone {
		detailInnerW := m.width - panePadding
		chromeH := lipgloss.Height(m.detailPaneChrome(detailInnerW))
		m.detail.SetSize(detailInnerW, max(innerH-chromeH, 1))
	} else {
		treeWidth := int(float64(m.width) * treeSplitRatio)
		detailWidth := m.width - treeWidth
		treeInnerW := treeWidth - panePadding
		detailInnerW := detailWidth - panePadding

		treeChromeH := lipgloss.Height(m.treePaneChrome(treeInnerW))
		detailChromeH := lipgloss.Height(m.detailPaneChrome(detailInnerW))

		m.tree.SetSize(treeInnerW, max(innerH-treeChromeH, 1))
		m.detail.SetSize(detailInnerW, max(innerH-detailChromeH, 1))
	}
}

// Run starts the TUI program.
func Run(result *compare.Result) error {
	m := New(result)
	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
