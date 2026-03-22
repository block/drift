package tui

import "charm.land/lipgloss/v2"

// Layout constants.
const (
	treeSplitRatio = 0.35 // tree pane width fraction
	panePadding    = 4    // border (2) + horizontal padding (2); used to compute inner content width
	borderHeight   = 2    // top + bottom border; used to compute inner content height
	minSplitWidth  = 80   // below this width, show one pane at a time
)

// Color palette - GitHub-inspired, muted and readable.
var (
	colorAdded     = lipgloss.Color("#3fb950") // green
	colorRemoved   = lipgloss.Color("#f85149") // red
	colorModified  = lipgloss.Color("#d29922") // amber
	colorUnchanged = lipgloss.Color("#484f58") // muted gray
	colorAccent    = lipgloss.Color("#58a6ff") // blue
	colorDim       = lipgloss.Color("#484f58") // dark muted
	colorSubtle    = lipgloss.Color("#6e7681") // lighter muted
	colorFg        = lipgloss.Color("#c9d1d9") // primary text
	colorBorder    = lipgloss.Color("#30363d") // border
	colorFocused   = lipgloss.Color("#58a6ff") // focused border

	colorSelectedBg = lipgloss.Color("#161b22") // selected row bg
	colorSelectedFg = lipgloss.Color("#f0f6fc") // selected row text
)

// Status text styles.
var (
	styleAdded    = lipgloss.NewStyle().Foreground(colorAdded)
	styleRemoved  = lipgloss.NewStyle().Foreground(colorRemoved)
	styleModified = lipgloss.NewStyle().Foreground(colorModified)
)

// Pane border styles.
var (
	styleFocusedBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorFocused).
				Padding(0, 1)

	styleBlurredBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)
)

// Text styles.
var (
	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorFg)

	styleHeaderLabel = lipgloss.NewStyle().
				Foreground(colorSubtle)

	styleDim = lipgloss.NewStyle().
			Foreground(colorDim)

	styleSubtle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	styleKindBadge = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	styleSummaryBar = lipgloss.NewStyle().
			Foreground(colorFg).
			Padding(0, 1)

	styleHelpBar = lipgloss.NewStyle().
			Padding(0, 1)
)

// Pane header styles.
var (
	stylePaneTitle = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Bold(true).
			PaddingRight(1)

	stylePaneTitleFocused = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true).
				PaddingRight(1)
)

// Tree styles.
var (
	styleTreeGuide = lipgloss.NewStyle().
			Foreground(colorBorder)

	styleSelectedBar = lipgloss.NewStyle().
				Background(colorSelectedBg).
				Foreground(colorSelectedFg).
				Bold(true)

	styleSelectedAccent = lipgloss.NewStyle().
				Background(colorSelectedBg).
				Foreground(colorAccent).
				Bold(true)

	styleNodeName = lipgloss.NewStyle().
			Foreground(colorFg)

	styleNodeDimName = lipgloss.NewStyle().
				Foreground(colorUnchanged)
)

// Detail / render styles.
var (
	styleHunkHeader = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	styleChangeKey  = lipgloss.NewStyle().Foreground(colorFg)

	styleChangeKeyBold = lipgloss.NewStyle().Foreground(colorFg).Bold(true)
	styleSectionName   = lipgloss.NewStyle().Foreground(colorAccent)

	styleErrorTitle = lipgloss.NewStyle().Foreground(colorRemoved).Bold(true)
	styleErrorBox   = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRemoved).
			Padding(1, 2)
)

// Alert overlay styles.
var (
	styleAlertSuccess = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAdded).
				Foreground(colorAdded).
				Bold(true).
				Padding(1, 3)

	styleAlertError = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorRemoved).
			Foreground(colorRemoved).
			Bold(true).
			Padding(1, 3)

	styleAlertInfo = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Foreground(colorAccent).
			Bold(true).
			Padding(1, 3)
)

// Search styles.
var (
	styleAccent = lipgloss.NewStyle().Foreground(colorAccent)

	styleSearchBar = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	styleSearchMatchChar = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f0f6fc")).
				Background(lipgloss.Color("#1c3a5e")).
				Bold(true)
)

// Tree connector characters (2-char wide for compact indentation).
const (
	treeBranch  = "├─"
	treeCorner  = "└─"
	treeBlank   = "  "
	treePipeGap = "│ "
)
