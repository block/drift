package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sahilm/fuzzy"
)

// searchModel manages an inline search bar within a pane.
type searchModel struct {
	input   textinput.Model
	active  bool
	query   string // confirmed or live query
	matches int    // number of matches found
}

func newSearchModel() searchModel {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "type to search…"
	ti.CharLimit = 256

	styles := ti.Styles()
	styles.Focused.Placeholder = styleDim
	styles.Focused.Text = lipgloss.NewStyle().Foreground(colorFg)
	styles.Blurred.Placeholder = styleDim
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(colorFg)
	ti.SetStyles(styles)

	return searchModel{input: ti}
}

// Activate enters search mode.
func (m *searchModel) Activate() tea.Cmd {
	m.active = true
	m.input.SetValue("")
	m.query = ""
	m.matches = 0
	return m.input.Focus()
}

// Deactivate exits search mode and clears the query.
func (m *searchModel) Deactivate() {
	m.active = false
	m.query = ""
	m.matches = 0
	m.input.SetValue("")
	m.input.Blur()
}

// Confirm exits search mode but keeps the query active.
func (m *searchModel) Confirm() {
	m.active = false
	m.query = m.input.Value()
	m.input.Blur()
}

// Active returns whether the search bar is actively receiving input.
func (m searchModel) Active() bool {
	return m.active
}

// Query returns the current search query (live while typing, confirmed after enter).
func (m searchModel) Query() string {
	if m.active {
		return m.input.Value()
	}
	return m.query
}

// HasQuery returns true if there's a non-empty search query.
func (m searchModel) HasQuery() bool {
	return m.Query() != ""
}

// SetMatches updates the match count display.
func (m *searchModel) SetMatches(n int) {
	m.matches = n
}

// Update handles input messages while search is active.
func (m searchModel) Update(msg tea.Msg) (searchModel, tea.Cmd) {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the search bar: bordered input box + match count below.
func (m searchModel) View(width int) string {
	// Inner width accounts for border (2) + padding (2).
	innerW := max(width-4, 10)

	// Icon + text input fills the entire box.
	icon := styleAccent.Render("⌕ ")
	iconW := ansi.StringWidth(icon)
	m.input.SetWidth(max(innerW-iconW-1, 5))
	input := m.input.View()

	box := styleSearchBar.Width(innerW).Render(icon + input)

	// Subtitle line below the box: esc hint on left, match count on right.
	hint := styleDim.Render("esc to close")
	var countLabel string
	if m.HasQuery() {
		if m.matches == 1 {
			countLabel = styleDim.Render("1 match")
		} else {
			countLabel = styleDim.Render(fmt.Sprintf("%d matches", m.matches))
		}
	}
	hintW := ansi.StringWidth(hint)
	countW := ansi.StringWidth(countLabel)
	gap := width - hintW - countW - 1
	var subtitle string
	if gap > 0 {
		subtitle = hint + strings.Repeat(" ", gap) + countLabel
	} else {
		subtitle = hint
	}

	return box + "\n" + subtitle
}

// --- Fuzzy matching utilities ---

// fuzzyHighlightLines runs fuzzy matching on each line of content
// and highlights matched characters in matching lines.
func fuzzyHighlightLines(content, query string) string {
	if query == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	// Strip ANSI for matching, keep original for rendering.
	stripped := make([]string, len(lines))
	for i, line := range lines {
		stripped[i] = ansi.Strip(line)
	}

	matches := fuzzy.Find(query, stripped)
	if len(matches) == 0 {
		return content
	}

	// Build a map of line index → matched char indices.
	matchMap := make(map[int][]int, len(matches))
	for _, m := range matches {
		matchMap[m.Index] = m.MatchedIndexes
	}

	for lineIdx, indices := range matchMap {
		lines[lineIdx] = highlightCharsInStyledLine(lines[lineIdx], indices)
	}

	return strings.Join(lines, "\n")
}

// fuzzyCountMatches counts lines that fuzzy-match the query.
func fuzzyCountMatches(content, query string) int {
	if query == "" {
		return 0
	}
	lines := strings.Split(content, "\n")
	stripped := make([]string, len(lines))
	for i, line := range lines {
		stripped[i] = ansi.Strip(line)
	}
	return len(fuzzy.Find(query, stripped))
}

// highlightCharsInStyledLine highlights specific plain-text character positions
// within an ANSI-styled line by walking the string and skipping escape sequences.
func highlightCharsInStyledLine(line string, indices []int) string {
	if len(indices) == 0 {
		return line
	}
	indexSet := make(map[int]bool, len(indices))
	for _, idx := range indices {
		indexSet[idx] = true
	}

	var result strings.Builder
	plainIdx := 0

	i := 0
	for i < len(line) {
		// Skip ANSI escape sequences.
		if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && line[j] != 'm' {
				j++
			}
			if j < len(line) {
				j++ // include 'm'
			}
			result.WriteString(line[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		if indexSet[plainIdx] {
			result.WriteString(styleSearchMatchChar.Render(string(r)))
		} else {
			result.WriteRune(r)
		}
		plainIdx++
		i += size
	}

	return result.String()
}

// highlightName renders a name with specific character indices highlighted.
func highlightName(name string, indices []int, baseStyle lipgloss.Style) string {
	if len(indices) == 0 {
		return baseStyle.Render(name)
	}
	indexSet := make(map[int]bool, len(indices))
	for _, idx := range indices {
		indexSet[idx] = true
	}
	hlStyle := baseStyle.Underline(true).Foreground(colorAccent)
	var b strings.Builder
	for i, r := range name {
		if indexSet[i] {
			b.WriteString(hlStyle.Render(string(r)))
		} else {
			b.WriteString(baseStyle.Render(string(r)))
		}
	}
	return b.String()
}
