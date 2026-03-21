package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// keyMap defines all keybindings for the app.
type keyMap struct {
	// Global
	Quit        key.Binding
	ToggleFocus key.Binding
	Help        key.Binding

	// Tree navigation
	Up         key.Binding
	Down       key.Binding
	Expand     key.Binding
	Collapse   key.Binding
	NextChange key.Binding
	PrevChange key.Binding

	// Detail scrolling
	PageDown key.Binding
	PageUp   key.Binding
	Top      key.Binding
	Bottom   key.Binding

	// Filtering
	Filter         key.Binding
	FilterAll      key.Binding
	FilterAdded    key.Binding
	FilterRemoved  key.Binding
	FilterModified key.Binding

	// Search
	Search key.Binding

	// Contextual
	Copy key.Binding
	Swap key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		ToggleFocus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Expand: key.NewBinding(
			key.WithKeys("enter", "right", "l"),
			key.WithHelp("→/enter", "expand"),
		),
		Collapse: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "collapse"),
		),
		NextChange: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next change"),
		),
		PrevChange: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "prev change"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		FilterAll: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "all"),
		),
		FilterAdded: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "added"),
		),
		FilterRemoved: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "removed"),
		),
		FilterModified: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "modified"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Copy: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy"),
			key.WithDisabled(),
		),
		Swap: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "swap A↔B"),
		),
	}
}

// ShortHelp returns the short help bindings shown by default.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Expand, k.Collapse, k.ToggleFocus, k.Filter, k.Search, k.Swap, k.Copy, k.Help, k.Quit}
}

// FullHelp returns the full help bindings shown when toggled.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Expand, k.Collapse},
		{k.NextChange, k.PrevChange, k.ToggleFocus},
		{k.Filter, k.FilterAll, k.FilterAdded, k.FilterRemoved, k.FilterModified},
		{k.PageDown, k.PageUp, k.Top, k.Bottom},
		{k.Search, k.Swap, k.Copy, k.Help, k.Quit},
	}
}
