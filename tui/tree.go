package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/block/drift/compare"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// treeItem is a flattened tree node for display in a list.
type treeItem struct {
	node               *compare.Node
	depth              int
	expanded           bool
	isLast             bool   // last child of its parent
	guides             []bool // for each ancestor depth: true = draw │ (has more siblings below)
	searchMatchIndexes []int  // matched character indices for fuzzy search highlighting
}

func (t treeItem) FilterValue() string { return t.node.Name }

// treeDelegate renders tree items with connectors, status icons, and kind badges.
type treeDelegate struct{}

func (d treeDelegate) Height() int                             { return 1 }
func (d treeDelegate) Spacing() int                            { return 0 }
func (d treeDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d treeDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ti, ok := item.(treeItem)
	if !ok {
		return
	}

	node := ti.node
	isSelected := index == m.Index()
	width := m.Width()

	// Shared content parts.
	badge := kindBadge(node.Kind)
	var delta string
	if node.Status == compare.Modified && !node.IsDir {
		if d := node.SizeDelta(); d != 0 {
			delta = formatSizeDelta(d)
		}
	}

	// maxW is the visual width available for the item (1 char reserved for the leading accent/space).
	maxW := width - 1

	if isSelected {
		// Plain-text prefix + content for full-width highlight.
		prefix := buildPrefix(ti, identity)
		content := statusIconChar(node.Status, node.IsDir, ti.expanded) + " " + nodeName(node)
		if badge != "" {
			content += " " + badge
		}
		if delta != "" {
			content += " " + delta
		}
		plain := prefix + content
		plain = ansi.Truncate(plain, maxW, "…")
		if pw := ansi.StringWidth(plain); pw < maxW {
			plain += strings.Repeat(" ", maxW-pw)
		}
		fmt.Fprint(w, styleSelectedAccent.Render("▍")+styleSelectedBar.Render(plain))
		return
	}

	// Non-selected: styled prefix + content. Leading space matches ▍ width.
	prefix := " " + buildPrefix(ti, func(s string) string { return styleTreeGuide.Render(s) })
	styledIcon := statusIcon(node.Status, node.IsDir, ti.expanded)

	var styledName string
	nameStyle := styleNodeName
	if node.Status == compare.Unchanged {
		nameStyle = styleNodeDimName
	}
	if len(ti.searchMatchIndexes) > 0 {
		styledName = highlightName(node.Name, ti.searchMatchIndexes, nameStyle)
	} else {
		styledName = nameStyle.Render(node.Name)
	}
	if node.IsDir {
		styledName += styleSubtle.Render("/")
	}

	line := prefix + styledIcon + " " + styledName
	if badge != "" {
		line += " " + styleKindBadge.Render(badge)
	}
	if delta != "" {
		line += " " + styleModified.Render(delta)
	}

	fmt.Fprint(w, ansi.Truncate(line, width, "…"))
}

// identity returns the string unchanged (used for plain-text prefix building).
func identity(s string) string { return s }

// buildPrefix constructs the tree connector prefix for a node.
// styleFn is applied to connector characters (identity for plain, styleTreeGuide.Render for styled).
func buildPrefix(ti treeItem, styleFn func(string) string) string {
	var b strings.Builder
	for _, guide := range ti.guides {
		if guide {
			b.WriteString(styleFn(treePipeGap))
		} else {
			b.WriteString(treeBlank)
		}
	}
	if ti.depth > 0 {
		if ti.isLast {
			b.WriteString(styleFn(treeCorner))
		} else {
			b.WriteString(styleFn(treeBranch))
		}
		b.WriteString(" ")
	}
	return b.String()
}

// nodeName returns the display name for a node (with trailing / for dirs).
func nodeName(node *compare.Node) string {
	if node.IsDir {
		return node.Name + "/"
	}
	return node.Name
}

// statusIconChar returns a plain icon character (no styling) for use in selected rows.
func statusIconChar(status compare.DiffStatus, isDir, expanded bool) string {
	if isDir {
		if expanded {
			return "⌄"
		}
		return "›"
	}
	switch status {
	case compare.Added:
		return "+"
	case compare.Removed:
		return "−"
	case compare.Modified:
		return "●"
	default:
		return "·"
	}
}

// statusIcon returns a colored icon for a node's status and dir state.
func statusIcon(status compare.DiffStatus, isDir, expanded bool) string {
	if isDir {
		arrow := "›"
		if expanded {
			arrow = "⌄"
		}
		switch status {
		case compare.Modified:
			return styleModified.Render(arrow)
		case compare.Added:
			return styleAdded.Render(arrow)
		case compare.Removed:
			return styleRemoved.Render(arrow)
		default:
			return styleDim.Render(arrow)
		}
	}

	switch status {
	case compare.Added:
		return styleAdded.Render("+")
	case compare.Removed:
		return styleRemoved.Render("−")
	case compare.Modified:
		return styleModified.Render("●")
	default:
		return styleDim.Render("·")
	}
}

// kindBadge returns a parenthesized label for non-text file kinds.
func kindBadge(kind compare.FileKind) string {
	switch kind {
	case compare.KindMachO:
		return "(binary)"
	case compare.KindPlist:
		return "(plist)"
	case compare.KindArchive:
		return "(archive)"
	case compare.KindDSYM:
		return "(dsym)"
	case compare.KindData:
		return "(data)"
	default:
		return ""
	}
}

// --- Tree model ---

// statusFilter controls which nodes are visible in the tree.
type statusFilter int

const (
	filterAll statusFilter = iota
	filterAdded
	filterRemoved
	filterModified
)

func (f statusFilter) String() string {
	switch f {
	case filterAll:
		return "all"
	case filterAdded:
		return "added"
	case filterRemoved:
		return "removed"
	case filterModified:
		return "modified"
	default:
		return "all"
	}
}

// matches returns true if a node should be visible under this filter.
func (f statusFilter) matches(status compare.DiffStatus) bool {
	switch f {
	case filterAll:
		return true
	case filterAdded:
		return status == compare.Added
	case filterRemoved:
		return status == compare.Removed
	case filterModified:
		return status == compare.Modified
	default:
		return true
	}
}

// treeModel manages the tree pane state.
type treeModel struct {
	list          list.Model
	root          *compare.Node
	items         []treeItem
	expanded      map[string]bool // path → expanded state
	keys          keyMap
	filter        statusFilter
	search        string           // raw search query
	searchMatches map[string][]int // path → matched char indices (nil = no search)
	showRoot      bool             // if true, include root node as a visible tree item
}

// nodeSelectedMsg is sent when a tree node is selected for detail viewing.
type nodeSelectedMsg struct {
	node *compare.Node
}

func newTreeModel(root *compare.Node, keys keyMap, width, height int, showRoot bool) treeModel {
	expanded := map[string]bool{"": true} // root is always expanded
	autoExpand(root, expanded)

	m := treeModel{
		root:     root,
		expanded: expanded,
		keys:     keys,
		showRoot: showRoot,
	}
	m.items = m.flattenTree()

	items := make([]list.Item, len(m.items))
	for i, ti := range m.items {
		items[i] = ti
	}

	delegate := treeDelegate{}
	l := list.New(items, delegate, width, height)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()
	l.InfiniteScrolling = false

	m.list = l
	return m
}

// autoExpand expands all directories that contain changes.
func autoExpand(node *compare.Node, expanded map[string]bool) {
	if node.IsDir && node.Status != compare.Unchanged {
		expanded[node.Path] = true
	}
	for _, child := range node.Children {
		autoExpand(child, expanded)
	}
}

// mouseScrollLines is how many cursor positions each scroll tick moves.
const mouseScrollLines = 3

func (m treeModel) Update(msg tea.Msg) (treeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Expand):
			return m.handleExpand()
		case key.Matches(msg, m.keys.Collapse):
			return m.handleCollapse()
		case key.Matches(msg, m.keys.NextChange):
			return m.jumpToChange(1)
		case key.Matches(msg, m.keys.PrevChange):
			return m.jumpToChange(-1)
		case key.Matches(msg, m.keys.Filter):
			return m.cycleFilter()
		case key.Matches(msg, m.keys.FilterAll):
			return m.setFilter(filterAll)
		case key.Matches(msg, m.keys.FilterAdded):
			return m.setFilter(filterAdded)
		case key.Matches(msg, m.keys.FilterRemoved):
			return m.setFilter(filterRemoved)
		case key.Matches(msg, m.keys.FilterModified):
			return m.setFilter(filterModified)
		}

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			for i := 0; i < mouseScrollLines; i++ {
				m.list.CursorUp()
			}
			return m, nil
		case tea.MouseWheelDown:
			for i := 0; i < mouseScrollLines; i++ {
				m.list.CursorDown()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m treeModel) View() string {
	return m.list.View()
}

// SetSize updates the tree pane dimensions.
func (m *treeModel) SetSize(width, height int) {
	m.list.SetSize(width, height)
}

// SelectedNode returns the currently selected node.
func (m treeModel) SelectedNode() *compare.Node {
	idx := m.list.Index()
	if idx >= 0 && idx < len(m.items) {
		return m.items[idx].node
	}
	return nil
}

// HandleClick selects the item at the given visible row and expands/collapses or selects it.
func (m treeModel) HandleClick(visibleRow int) (treeModel, tea.Cmd) {
	itemIndex := m.list.Paginator.Page*m.list.Paginator.PerPage + visibleRow
	if itemIndex < 0 || itemIndex >= len(m.items) {
		return m, nil
	}

	m.list.Select(itemIndex)
	node := m.items[itemIndex].node

	if node.IsDir {
		// Root node is always expanded - don't toggle it.
		if m.showRoot && node.Path == "" {
			return m, nil
		}
		// Toggle expand/collapse.
		if m.expanded[node.Path] {
			delete(m.expanded, node.Path)
		} else {
			m.expanded[node.Path] = true
		}
		m.rebuildItems()
		return m, nil
	}

	// File: emit selection for detail loading.
	return m, func() tea.Msg {
		return nodeSelectedMsg{node: node}
	}
}

func (m treeModel) handleExpand() (treeModel, tea.Cmd) {
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.items) {
		return m, nil
	}

	item := m.items[idx]
	node := item.node

	if node.IsDir {
		m.expanded[node.Path] = true
		m.rebuildItems()
		return m, nil
	}

	// Non-directory: emit selection for detail loading.
	return m, func() tea.Msg {
		return nodeSelectedMsg{node: node}
	}
}

func (m treeModel) handleCollapse() (treeModel, tea.Cmd) {
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.items) {
		return m, nil
	}

	item := m.items[idx]
	node := item.node

	if node.IsDir && m.expanded[node.Path] {
		// Root node is always expanded - don't collapse it.
		if !(m.showRoot && node.Path == "") {
			delete(m.expanded, node.Path)
			m.rebuildItems()
			return m, nil
		}
	}

	// Move cursor to parent directory.
	for i := idx - 1; i >= 0; i-- {
		if m.items[i].node.IsDir && m.items[i].depth < item.depth {
			m.list.Select(i)
			break
		}
	}
	return m, nil
}

// filterChangedMsg is emitted when the tree filter changes.
type filterChangedMsg struct{}

const statusFilterCount = 4 // filterAll, filterAdded, filterRemoved, filterModified

func (m treeModel) cycleFilter() (treeModel, tea.Cmd) {
	next := (m.filter + 1) % statusFilterCount
	return m.setFilter(next)
}

func (m treeModel) setFilter(f statusFilter) (treeModel, tea.Cmd) {
	if m.filter == f {
		return m, nil
	}
	m.filter = f
	m.rebuildItems()
	return m, func() tea.Msg { return filterChangedMsg{} }
}

// Filter returns the current status filter label.
func (m treeModel) Filter() statusFilter {
	return m.filter
}

// SetSearch updates the search query using fuzzy matching and rebuilds the tree.
func (m *treeModel) SetSearch(query string) {
	m.search = query
	if query == "" {
		m.searchMatches = nil
	} else {
		// Collect all leaf node names and paths for fuzzy matching.
		var names, paths []string
		collectLeaves(m.root, &names, &paths)

		matches := fuzzy.Find(query, names)
		m.searchMatches = make(map[string][]int, len(matches))
		for _, match := range matches {
			m.searchMatches[paths[match.Index]] = match.MatchedIndexes
		}
	}
	m.rebuildItems()
}

// SearchMatches returns the number of fuzzy-matched items.
func (m treeModel) SearchMatches() int {
	if m.searchMatches != nil {
		return len(m.searchMatches)
	}
	count := 0
	for _, item := range m.items {
		if !item.node.IsDir {
			count++
		}
	}
	return count
}

// collectLeaves recursively collects non-directory node names and paths.
func collectLeaves(node *compare.Node, names, paths *[]string) {
	if !node.IsDir {
		*names = append(*names, node.Name)
		*paths = append(*paths, node.Path)
		return
	}
	for _, child := range node.Children {
		collectLeaves(child, names, paths)
	}
}

func (m treeModel) jumpToChange(direction int) (treeModel, tea.Cmd) {
	idx := m.list.Index()
	count := len(m.items)
	if count == 0 {
		return m, nil
	}

	for i := 1; i < count; i++ {
		next := (idx + i*direction + count) % count
		node := m.items[next].node
		if !node.IsDir && node.Status != compare.Unchanged {
			m.list.Select(next)
			break
		}
	}
	return m, nil
}

func (m *treeModel) rebuildItems() {
	cursor := m.list.Index()
	var selectedPath string
	if cursor >= 0 && cursor < len(m.items) {
		selectedPath = m.items[cursor].node.Path
	}

	m.items = m.flattenTree()

	items := make([]list.Item, len(m.items))
	for i, ti := range m.items {
		items[i] = ti
	}
	m.list.SetItems(items)

	for i, ti := range m.items {
		if ti.node.Path == selectedPath {
			m.list.Select(i)
			return
		}
	}
}

// nodeVisible returns true if a node passes the current status filter and search query.
// Directories are visible if they have any visible descendants.
func (m treeModel) nodeVisible(node *compare.Node) bool {
	if !node.IsDir {
		if m.filter != filterAll && !m.filter.matches(node.Status) {
			return false
		}
		if m.searchMatches != nil {
			_, matched := m.searchMatches[node.Path]
			return matched
		}
		return true
	}
	for _, child := range node.Children {
		if m.nodeVisible(child) {
			return true
		}
	}
	return false
}

// flattenTree walks the tree respecting expand state and filter, producing
// a flat list with tree connector metadata (guides, isLast) for rendering.
func (m treeModel) flattenTree() []treeItem {
	var items []treeItem
	var walk func(node *compare.Node, depth int, guides []bool, isLast bool)
	walk = func(node *compare.Node, depth int, guides []bool, isLast bool) {
		if depth >= 0 {
			g := make([]bool, len(guides))
			copy(g, guides)

			items = append(items, treeItem{
				node:               node,
				depth:              depth,
				expanded:           m.expanded[node.Path],
				isLast:             isLast,
				guides:             g,
				searchMatchIndexes: m.searchMatches[node.Path],
			})
		}

		if node.IsDir && m.expanded[node.Path] {
			// Filter visible children.
			var visible []*compare.Node
			for _, child := range node.Children {
				if m.nodeVisible(child) {
					visible = append(visible, child)
				}
			}
			for i, child := range visible {
				last := i == len(visible)-1
				childGuides := guides
				if depth >= 0 {
					childGuides = append(append([]bool{}, guides...), !isLast)
				}
				walk(child, depth+1, childGuides, last)
			}
		}
	}

	startDepth := -1
	if m.showRoot {
		startDepth = 0
	}
	walk(m.root, startDepth, nil, true)
	return items
}
