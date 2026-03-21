package tui

import (
	"runtime"
	"strings"

	"github.com/block/drift/compare"
)

// ErrorView renders a structured error display for a failed detail load.
type ErrorView struct {
	Node  *compare.Node
	Err   error
	Width int
}

func (v ErrorView) CopyableText() string {
	header := NodeHeaderView{Node: v.Node, Width: v.Width}.CopyableText()
	return header + "\nError:  " + v.Err.Error() + "\n"
}

func (v ErrorView) Render() string {
	var b strings.Builder

	b.WriteString(NodeHeaderView{Node: v.Node, Width: v.Width}.Render())
	b.WriteString("\n\n")

	var box strings.Builder
	box.WriteString(styleErrorTitle.Render("Error loading details"))
	box.WriteString("\n\n")
	box.WriteString(field("Reason", wrapText(v.Err.Error(), max(v.Width-16, 20))))
	box.WriteString("\n")
	box.WriteString(field("Path", styleSubtle.Render(v.Node.Path)))
	box.WriteString(field("Kind", styleSubtle.Render(v.Node.Kind.String())))

	if v.Node.Kind == compare.KindMachO {
		box.WriteString("\n")
		box.WriteString(styleDim.Render("Binary analysis requires nm and size."))
		box.WriteString("\n")
		switch runtime.GOOS {
		case "darwin":
			box.WriteString(styleDim.Render("Install Xcode Command Line Tools: xcode-select --install"))
		case "linux":
			box.WriteString(styleDim.Render("Install binutils via your package manager."))
		default:
			box.WriteString(styleDim.Render("These tools are not available on this platform."))
		}
	}

	b.WriteString(styleErrorBox.Width(max(v.Width-6, 10)).Render(box.String()))
	return b.String()
}
