package compare

import "encoding/json"

// DiffStatus represents the diff status of a node.
type DiffStatus int

const (
	Unchanged DiffStatus = iota
	Added
	Removed
	Modified
)

var diffStatusNames = [...]string{"unchanged", "added", "removed", "modified"}

func (s DiffStatus) String() string {
	if int(s) < len(diffStatusNames) {
		return diffStatusNames[s]
	}
	return "unknown"
}

func (s DiffStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// FileKind represents the detected type of a file.
type FileKind int

const (
	KindUnknown FileKind = iota
	KindDirectory
	KindArchive
	KindMachO
	KindPlist
	KindDSYM
	KindText
	KindImage // image files (png, jpg, gif, etc.)
	KindData  // opaque binary data (code signatures, assets, etc.)
)

var fileKindNames = [...]string{"unknown", "directory", "archive", "macho", "plist", "dsym", "text", "image", "data"}

func (k FileKind) String() string {
	if int(k) < len(fileKindNames) {
		return fileKindNames[k]
	}
	return "unknown"
}

func (k FileKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// Node represents a single entry in the diff tree.
type Node struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Status   DiffStatus `json:"status"`
	Kind     FileKind   `json:"kind"`
	IsDir    bool       `json:"is_dir"`
	SizeA    int64      `json:"size_a"`
	SizeB    int64      `json:"size_b"`
	Children []*Node    `json:"children,omitempty"`
}

// SizeDelta returns the size difference between B and A.
func (n *Node) SizeDelta() int64 {
	return n.SizeB - n.SizeA
}

// GitCommitInfo holds metadata for a single git commit.
type GitCommitInfo struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Body    string `json:"body,omitempty"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Ref     string `json:"ref"` // original ref (branch name, tag, HEAD, etc.)
	Remote  string `json:"remote,omitempty"`
}

// GitMeta holds git-specific metadata for the comparison.
type GitMeta struct {
	CommitA *GitCommitInfo `json:"commit_a,omitempty"`
	CommitB *GitCommitInfo `json:"commit_b,omitempty"`
}

// Result is the top-level output of a comparison.
type Result struct {
	PathA   string   `json:"path_a"`
	PathB   string   `json:"path_b"`
	Mode    Mode     `json:"mode"`
	Root    *Node    `json:"root"`
	Summary Summary  `json:"summary"`
	Git     *GitMeta `json:"git,omitempty"`
}

// Summary aggregates diff statistics.
type Summary struct {
	Added     int   `json:"added"`
	Removed   int   `json:"removed"`
	Modified  int   `json:"modified"`
	Unchanged int   `json:"unchanged"`
	SizeDelta int64 `json:"size_delta"`
}

// ComputeSummary walks the tree and computes aggregate statistics.
func ComputeSummary(root *Node) Summary {
	var s Summary
	var walk func(*Node)
	walk = func(n *Node) {
		if !n.IsDir {
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
	walk(root)
	return s
}

// fileEntry is an internal representation of a file during comparison.
type fileEntry struct {
	relPath string
	size    int64
	isDir   bool
}
