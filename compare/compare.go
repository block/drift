package compare

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Mode represents a comparison mode.
type Mode = string

const (
	ModeTree   Mode = "tree"
	ModeBinary Mode = "binary"
	ModePlist  Mode = "plist"
	ModeText   Mode = "text"
)

// Compare runs the appropriate comparison for the given paths and mode.
// If mode is empty, it is auto-detected from the inputs.
func Compare(pathA, pathB, mode Mode) (*Result, error) {
	if mode == "" {
		detected, err := detectMode(pathA, pathB)
		if err != nil {
			return nil, err
		}
		mode = detected
	}

	var root *Node
	var err error

	switch mode {
	case ModeTree:
		root, err = compareTree(pathA, pathB)
	case ModeBinary:
		root, err = compareSingle(pathA, pathB, KindMachO)
	case ModePlist:
		root, err = compareSingle(pathA, pathB, KindPlist)
	case ModeText:
		root, err = compareSingle(pathA, pathB, KindText)
	default:
		return nil, fmt.Errorf("unknown mode: %s (valid: tree, binary, plist, text)", mode)
	}
	if err != nil {
		return nil, err
	}

	return &Result{
		PathA:   pathA,
		PathB:   pathB,
		Mode:    mode,
		Root:    root,
		Summary: ComputeSummary(root),
	}, nil
}

// compareSingle builds a single-node tree for standalone file comparison.
// It returns the node plus the source directories to use as PathA/PathB in the
// Result, so that Detail() can locate files via readContent(source, relPath).
func compareSingle(pathA, pathB string, kind FileKind) (*Node, error) {
	infoA, err := os.Stat(pathA)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", pathA, err)
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", pathB, err)
	}

	node := &Node{
		Name:  filepath.Base(pathA) + " ↔ " + filepath.Base(pathB),
		Path:  filepath.Base(pathA),
		Kind:  kind,
		SizeA: infoA.Size(),
		SizeB: infoB.Size(),
	}

	if infoA.Size() != infoB.Size() {
		node.Status = Modified
	} else {
		hashA, errA := hashFileOnDisk(pathA)
		hashB, errB := hashFileOnDisk(pathB)
		if errA == nil && errB == nil && hashA != hashB {
			node.Status = Modified
		}
	}

	return node, nil
}

func compareTree(pathA, pathB string) (*Node, error) {
	entriesA, err := readEntries(pathA)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", pathA, err)
	}
	entriesB, err := readEntries(pathB)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", pathB, err)
	}

	rootName := filepath.Base(pathA) + " ↔ " + filepath.Base(pathB)
	return diffEntries(entriesA, entriesB, rootName, pathA, pathB), nil
}

// readEntries dispatches to the appropriate reader based on input type.
func readEntries(path string) ([]fileEntry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return walkDir(path)
	}
	if isArchive(path) {
		return readArchive(path)
	}
	return nil, fmt.Errorf("%s: not a directory or supported archive", path)
}

// diffEntries compares two sets of file entries and builds a diff tree.
// sourceA and sourceB are the original paths (directories or archives) used
// for content hashing when file sizes match.
func diffEntries(entriesA, entriesB []fileEntry, rootName, sourceA, sourceB string) *Node {
	mapA := indexEntries(entriesA)
	mapB := indexEntries(entriesB)

	allPaths := make(map[string]struct{})
	for p := range mapA {
		allPaths[p] = struct{}{}
	}
	for p := range mapB {
		allPaths[p] = struct{}{}
	}

	var nodes []*Node
	for p := range allPaths {
		a, inA := mapA[p]
		b, inB := mapB[p]

		n := &Node{
			Name: filepath.Base(p),
			Path: p,
		}

		switch {
		case inA && !inB:
			n.Status = Removed
			n.IsDir = a.isDir
			n.SizeA = a.size
			n.Kind = classifyPath(p, a.isDir)
		case !inA && inB:
			n.Status = Added
			n.IsDir = b.isDir
			n.SizeB = b.size
			n.Kind = classifyPath(p, b.isDir)
		default:
			n.IsDir = a.isDir || b.isDir
			n.SizeA = a.size
			n.SizeB = b.size
			if n.IsDir {
				n.Status = Unchanged
			} else if a.size != b.size {
				n.Status = Modified
			} else {
				// Same size: compare content hashes to detect modifications.
				n.Status = contentStatus(sourceA, sourceB, p)
			}
			n.Kind = classifyPath(p, n.IsDir)
		}

		nodes = append(nodes, n)
	}

	return buildTree(nodes, rootName)
}

// contentStatus compares SHA-256 hashes of a file in both sources.
// Falls back to Unchanged if hashing fails (preserving size-only behavior).
func contentStatus(sourceA, sourceB, relPath string) DiffStatus {
	hashA, errA := contentHash(sourceA, relPath)
	hashB, errB := contentHash(sourceB, relPath)
	if errA != nil || errB != nil {
		return Unchanged
	}
	if hashA != hashB {
		return Modified
	}
	return Unchanged
}

func indexEntries(entries []fileEntry) map[string]fileEntry {
	m := make(map[string]fileEntry, len(entries))
	for _, e := range entries {
		m[e.relPath] = e
	}
	return m
}

// classifyPath determines the FileKind based on path and directory status.
func classifyPath(path string, isDir bool) FileKind {
	if isDir {
		if strings.HasSuffix(strings.ToLower(path), ".dsym") {
			return KindDSYM
		}
		return KindDirectory
	}
	if isArchive(path) {
		return KindArchive
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".plist":
		return KindPlist
	case ".dylib", ".a", ".o":
		return KindMachO
	}

	// Known binary/opaque data extensions.
	if isDataExt(ext) {
		return KindData
	}

	// Binaries inside .framework or .app bundles are extensionless files
	// that are direct children of the bundle directory.
	base := filepath.Base(path)
	parent := filepath.Base(filepath.Dir(path))
	if !strings.Contains(base, ".") && (strings.HasSuffix(parent, ".framework") || strings.HasSuffix(parent, ".app")) {
		return KindMachO
	}

	// Default to text - compareText performs content-based binary detection
	// at diff time and returns ErrBinaryContent for misclassified files.
	return KindText
}

// isDataExt returns true for extensions that are known binary/opaque data.
func isDataExt(ext string) bool {
	switch ext {
	case ".car", ".nib", ".storyboardc", ".mom", ".momd", ".omo",
		".metallib", ".dat", ".db", ".sqlite", ".png", ".jpg", ".jpeg",
		".gif", ".icns", ".tiff", ".tif", ".pdf", ".ttf", ".otf",
		".woff", ".woff2", ".p12", ".cer", ".der", ".mobileprovision",
		".lproj", ".sig", ".bin", ".enc", ".bom", ".pak":
		return true
	}
	return false
}

// buildTree constructs a hierarchical tree from a flat list of nodes.
func buildTree(nodes []*Node, rootName string) *Node {
	root := &Node{
		Name:  rootName,
		Path:  "",
		IsDir: true,
		Kind:  KindDirectory,
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Path < nodes[j].Path
	})

	dirs := map[string]*Node{"": root}
	for _, n := range nodes {
		if n.IsDir {
			dirs[n.Path] = n
		}
	}

	for _, n := range nodes {
		parentPath := filepath.Dir(n.Path)
		if parentPath == "." {
			parentPath = ""
		}
		parent := ensureParent(dirs, root, parentPath)
		parent.Children = append(parent.Children, n)
	}

	propagateStatus(root)
	return root
}

// ensureParent creates intermediate directory nodes as needed.
func ensureParent(dirs map[string]*Node, root *Node, path string) *Node {
	if n, ok := dirs[path]; ok {
		return n
	}
	parentPath := filepath.Dir(path)
	if parentPath == "." {
		parentPath = ""
	}
	parent := ensureParent(dirs, root, parentPath)
	n := &Node{
		Name:   filepath.Base(path),
		Path:   path,
		IsDir:  true,
		Kind:   KindDirectory,
		Status: Unchanged,
	}
	dirs[path] = n
	parent.Children = append(parent.Children, n)
	return n
}

// propagateStatus sets directory statuses based on children and aggregates sizes.
func propagateStatus(n *Node) {
	if !n.IsDir || len(n.Children) == 0 {
		return
	}
	for _, c := range n.Children {
		propagateStatus(c)
	}

	// Sort: directories first, then alphabetically.
	sort.Slice(n.Children, func(i, j int) bool {
		ci, cj := n.Children[i], n.Children[j]
		if ci.IsDir != cj.IsDir {
			return ci.IsDir
		}
		return ci.Name < cj.Name
	})

	hasChanges := false
	var totalA, totalB int64
	for _, c := range n.Children {
		if c.Status != Unchanged {
			hasChanges = true
		}
		totalA += c.SizeA
		totalB += c.SizeB
	}
	if hasChanges && n.Status == Unchanged {
		n.Status = Modified
	}
	// Always aggregate: a directory's size is the sum of its children,
	// not the filesystem directory entry metadata size.
	n.SizeA = totalA
	n.SizeB = totalB
}

// --- Mode Detection ---

func detectMode(pathA, pathB string) (Mode, error) {
	infoA, err := os.Stat(pathA)
	if err != nil {
		return "", fmt.Errorf("cannot access %s: %w", pathA, err)
	}
	infoB, err := os.Stat(pathB)
	if err != nil {
		return "", fmt.Errorf("cannot access %s: %w", pathB, err)
	}

	if infoA.IsDir() && infoB.IsDir() {
		return ModeTree, nil
	}

	if !infoA.IsDir() && !infoB.IsDir() {
		extA := strings.ToLower(filepath.Ext(pathA))
		extB := strings.ToLower(filepath.Ext(pathB))

		if isArchive(pathA) && isArchive(pathB) {
			return ModeTree, nil
		}
		if extA == ".plist" && extB == ".plist" {
			return ModePlist, nil
		}
		if isMachO(pathA) && isMachO(pathB) {
			return ModeBinary, nil
		}
		return ModeText, nil
	}

	// Mixed (dir + archive, etc.) → tree
	return ModeTree, nil
}

// Mach-O magic numbers for binary detection.
const (
	machoMagic32      = 0xFEEDFACE // 32-bit Mach-O
	machoMagic64      = 0xFEEDFACF // 64-bit Mach-O
	machoFatMagic     = 0xCAFEBABE // Fat/universal binary
	machoMagic32Swap  = 0xCEFAEDFE // Byte-swapped 32-bit
	machoMagic64Swap  = 0xCFFAEDFE // Byte-swapped 64-bit
	machoFatMagicSwap = 0xBEBAFECA // Byte-swapped fat
)

// isMachO checks if a file starts with a Mach-O magic number.
func isMachO(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	var magic [4]byte
	if _, err := f.Read(magic[:]); err != nil {
		return false
	}

	m := uint32(magic[0])<<24 | uint32(magic[1])<<16 | uint32(magic[2])<<8 | uint32(magic[3])
	switch m {
	case machoMagic32, machoMagic64,
		machoFatMagic,
		machoMagic32Swap, machoMagic64Swap,
		machoFatMagicSwap:
		return true
	}
	return false
}
