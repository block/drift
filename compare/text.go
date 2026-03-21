package compare

import (
	"fmt"
	"net/http"
	"strings"

	"znkr.io/diff/textdiff"
)

// ErrBinaryContent indicates that a file classified as text actually
// contains binary data and should not be diffed as text.
var ErrBinaryContent = fmt.Errorf("file contains binary content")

// TextDiff contains a unified diff between two text files.
type TextDiff struct {
	Hunks []TextHunk `json:"hunks"`
}

// TextHunk represents a single hunk in a unified diff.
type TextHunk struct {
	OldStart int    `json:"old_start"`
	OldCount int    `json:"old_count"`
	NewStart int    `json:"new_start"`
	NewCount int    `json:"new_count"`
	Lines    []Line `json:"lines"`
}

// Line represents a single line in a diff hunk.
type Line struct {
	Kind    string `json:"kind"` // "context", "added", "removed"
	Content string `json:"content"`
}

// compareText reads two text files and produces a unified diff.
func compareText(sourceA, sourceB, relPath string, status DiffStatus) (*TextDiff, error) {
	var textA, textB string

	if status != Added {
		dataA, err := readContent(sourceA, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading text A: %w", err)
		}
		if isBinaryData(dataA) {
			return nil, ErrBinaryContent
		}
		textA = string(dataA)
	}
	if status != Removed {
		dataB, err := readContent(sourceB, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading text B: %w", err)
		}
		if isBinaryData(dataB) {
			return nil, ErrBinaryContent
		}
		textB = string(dataB)
	}

	unified := textdiff.Unified(textA, textB)
	if unified == "" {
		return &TextDiff{}, nil
	}

	hunks, err := parseUnifiedDiff(unified)
	if err != nil {
		return nil, err
	}
	return &TextDiff{Hunks: hunks}, nil
}

// isBinaryData uses net/http.DetectContentType (MIME sniffing) to detect
// binary content that shouldn't be diffed as text.
func isBinaryData(data []byte) bool {
	ct := http.DetectContentType(data)
	return !strings.HasPrefix(ct, "text/")
}

// parseUnifiedDiff parses unified diff output into structured hunks.
func parseUnifiedDiff(diff string) ([]TextHunk, error) {
	var hunks []TextHunk
	var current *TextHunk

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") {
			h := TextHunk{}
			_, err := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@",
				&h.OldStart, &h.OldCount, &h.NewStart, &h.NewCount)
			if err != nil {
				// Try single-line hunk format: @@ -N +N,M @@
				_, err = fmt.Sscanf(line, "@@ -%d +%d,%d @@",
					&h.OldStart, &h.NewStart, &h.NewCount)
				if err != nil {
					// Try: @@ -N,M +N @@
					_, err = fmt.Sscanf(line, "@@ -%d,%d +%d @@",
						&h.OldStart, &h.OldCount, &h.NewStart)
					if err != nil {
						// Try: @@ -N +N @@
						_, err = fmt.Sscanf(line, "@@ -%d +%d @@",
							&h.OldStart, &h.NewStart)
						if err != nil {
							continue
						}
					}
				}
			}
			hunks = append(hunks, h)
			current = &hunks[len(hunks)-1]
			continue
		}

		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "-"):
			current.Lines = append(current.Lines, Line{Kind: "removed", Content: line[1:]})
		case strings.HasPrefix(line, "+"):
			current.Lines = append(current.Lines, Line{Kind: "added", Content: line[1:]})
		case strings.HasPrefix(line, " "):
			current.Lines = append(current.Lines, Line{Kind: "context", Content: line[1:]})
		}
	}

	return hunks, nil
}
