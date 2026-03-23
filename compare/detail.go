package compare

import (
	"errors"
	"fmt"
)

// DetailResult holds an on-demand detail diff for a single node.
type DetailResult struct {
	Kind   FileKind    `json:"kind"`
	Plist  *PlistDiff  `json:"plist,omitempty"`
	Binary *BinaryDiff `json:"binary,omitempty"`
	Text   *TextDiff   `json:"text,omitempty"`
	Image  *ImageDiff  `json:"image,omitempty"`
	Dir    *DirSummary `json:"dir,omitempty"`
}

// DirSummary holds aggregate statistics for a directory node.
type DirSummary struct {
	TotalFiles int   `json:"total_files"`
	Added      int   `json:"added"`
	Removed    int   `json:"removed"`
	Modified   int   `json:"modified"`
	Unchanged  int   `json:"unchanged"`
	SizeDelta  int64 `json:"size_delta"`
}

// Detail computes an on-demand detail diff for a specific node.
// It reads file content from the sources referenced in the result.
func Detail(result *Result, node *Node) (*DetailResult, error) {
	// Resolve source paths. For git mode, prefix with git:: so that
	// readContent/contentHash/prepareBinaryPath dispatch correctly.
	sourceA, sourceB := result.PathA, result.PathB
	if result.Mode == ModeGit {
		sourceA, sourceB = gitSourcePaths(result)
	}

	switch node.Kind {
	case KindDirectory, KindDSYM:
		return &DetailResult{Kind: node.Kind, Dir: summarizeDir(node)}, nil
	case KindPlist:
		diff, err := comparePlist(sourceA, sourceB, node.Path, node.Status)
		if err != nil {
			return nil, err
		}
		return &DetailResult{Kind: KindPlist, Plist: diff}, nil
	case KindMachO:
		diff, err := compareBinary(sourceA, sourceB, node.Path, node.Status)
		if err != nil {
			return nil, err
		}
		return &DetailResult{Kind: KindMachO, Binary: diff}, nil
	case KindText:
		diff, err := compareText(sourceA, sourceB, node.Path, node.Status)
		if err != nil {
			if errors.Is(err, ErrBinaryContent) {
				// File was classified as text but contains binary data.
				return &DetailResult{Kind: KindData}, nil
			}
			return nil, err
		}
		return &DetailResult{Kind: KindText, Text: diff}, nil
	case KindImage:
		diff, err := compareImage(sourceA, sourceB, node.Path, node.Status)
		if err != nil {
			return nil, err
		}
		return &DetailResult{Kind: KindImage, Image: diff}, nil
	case KindData:
		return &DetailResult{Kind: KindData}, nil
	default:
		return nil, fmt.Errorf("detail not yet supported for kind: %s", node.Kind)
	}
}

// summarizeDir walks a directory node's children to produce aggregate stats.
func summarizeDir(node *Node) *DirSummary {
	s := &DirSummary{}
	var walk func(*Node)
	walk = func(n *Node) {
		if !n.IsDir {
			s.TotalFiles++
			switch n.Status {
			case Added:
				s.Added++
			case Removed:
				s.Removed++
			case Modified:
				s.Modified++
			case Unchanged:
				s.Unchanged++
			}
			s.SizeDelta += n.SizeDelta()
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, c := range node.Children {
		walk(c)
	}
	return s
}
