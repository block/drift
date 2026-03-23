package tui

import (
	"fmt"
	"strings"

	"github.com/block/drift/compare"
)

// GitMetaView renders git commit metadata for the detail pane.
// It renders its own header instead of using NodeHeaderView.
type GitMetaView struct {
	Git   *compare.GitMeta
	Dir   *compare.DirSummary
	Width int
}

func (v GitMetaView) Render() string {
	var b strings.Builder

	// Compact summary line.
	if v.Dir != nil {
		b.WriteString(renderGitSummaryLine(v.Dir))
		b.WriteString("\n")
		b.WriteString(divider(v.Width))
		b.WriteString("\n\n")
	}

	if v.Git != nil {
		if c := v.Git.CommitA; c != nil {
			b.WriteString(renderCommitCompact(c))
			b.WriteString("\n")
		}
		if c := v.Git.CommitB; c != nil {
			b.WriteString(renderCommitCompact(c))
		}
	}

	return b.String()
}

func (v GitMetaView) CopyableText() string {
	var b strings.Builder

	if v.Dir != nil {
		fmt.Fprintf(&b, "%d files  +%d added  -%d removed  ~%d modified",
			v.Dir.TotalFiles, v.Dir.Added, v.Dir.Removed, v.Dir.Modified)
		if v.Dir.SizeDelta != 0 {
			fmt.Fprintf(&b, "  %s", formatSizeDelta(v.Dir.SizeDelta))
		}
		b.WriteString("\n\n")
	}

	if v.Git != nil {
		if c := v.Git.CommitA; c != nil {
			b.WriteString(commitPlainText(c))
		}
		if c := v.Git.CommitB; c != nil {
			b.WriteString(commitPlainText(c))
		}
	}

	return b.String()
}

// renderGitSummaryLine renders a compact one-line summary of changes.
func renderGitSummaryLine(d *compare.DirSummary) string {
	var parts []string
	parts = append(parts, styleDim.Render(fmt.Sprintf("%d files", d.TotalFiles)))
	if d.Added > 0 {
		parts = append(parts, styleAdded.Render(fmt.Sprintf("+%d added", d.Added)))
	}
	if d.Removed > 0 {
		parts = append(parts, styleRemoved.Render(fmt.Sprintf("-%d removed", d.Removed)))
	}
	if d.Modified > 0 {
		parts = append(parts, styleModified.Render(fmt.Sprintf("~%d modified", d.Modified)))
	}

	line := strings.Join(parts, styleDim.Render("  "))
	if d.SizeDelta != 0 {
		line += "  " + styleModified.Render(formatSizeDelta(d.SizeDelta))
	}
	return line
}

// renderCommitCompact renders a single commit in compact form:
//
//	ref  sha  author  date
//	subject line
//	PR  url
func renderCommitCompact(c *compare.GitCommitInfo) string {
	var b strings.Builder

	// Line 1: ref  sha  author  date
	ref := c.Ref
	if ref == c.SHA {
		ref = ""
	}
	sep := styleDim.Render(" · ")

	var meta []string
	if ref != "" {
		meta = append(meta, styleTitle.Render(ref))
	}
	meta = append(meta, styleAccent.Render(shortSHA(c.SHA)))
	meta = append(meta, styleSubtle.Render(c.Author))
	meta = append(meta, styleDim.Render(shortDate(c.Date)))
	b.WriteString(strings.Join(meta, sep))
	b.WriteString("\n")

	// Line 2: subject
	b.WriteString(styleChangeKey.Render(c.Subject))
	b.WriteString("\n")

	// Line 3: PR link (preferred) or commit link.
	if url := c.PRURL(); url != "" {
		b.WriteString(styleDim.Render("PR ") + styleAccent.Render(url))
		b.WriteString("\n")
	} else if url := c.CommitURL(); url != "" {
		b.WriteString(styleAccent.Render(url))
		b.WriteString("\n")
	}

	return b.String()
}

// shortDate extracts just the date portion (YYYY-MM-DD) from a git date string.
func shortDate(date string) string {
	if len(date) >= 10 {
		return date[:10]
	}
	return date
}

func commitPlainText(c *compare.GitCommitInfo) string {
	var b strings.Builder
	ref := c.Ref
	if ref == c.SHA {
		ref = shortSHA(c.SHA)
	}
	fmt.Fprintf(&b, "%s  %s  %s  %s\n", ref, shortSHA(c.SHA), c.Author, shortDate(c.Date))
	fmt.Fprintf(&b, "%s\n", c.Subject)
	if url := c.PRURL(); url != "" {
		fmt.Fprintf(&b, "PR %s\n", url)
	} else if url := c.CommitURL(); url != "" {
		fmt.Fprintf(&b, "%s\n", url)
	}
	b.WriteString("\n")
	return b.String()
}
