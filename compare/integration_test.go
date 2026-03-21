package compare

import (
	"path/filepath"
	"testing"
)

// testdataDir returns the path to the testdata directory.
// Go test sets the working directory to the package directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("cannot resolve testdata path: %v", err)
	}
	return abs
}

// findNode walks the tree and returns the first node matching the given path.
func findNode(root *Node, path string) *Node {
	if root.Path == path {
		return root
	}
	for _, c := range root.Children {
		if n := findNode(c, path); n != nil {
			return n
		}
	}
	return nil
}

func TestIntegration_DirectoryDiff(t *testing.T) {
	td := testdataDir(t)
	pathA := filepath.Join(td, "app-v1")
	pathB := filepath.Join(td, "app-v2")

	result, err := Compare(pathA, pathB, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Mode != "tree" {
		t.Fatalf("mode = %q, want tree", result.Mode)
	}

	// --- Summary ---
	s := result.Summary
	if s.Added != 2 {
		t.Errorf("summary.added = %d, want 2 (NewFeature binary + plist)", s.Added)
	}
	if s.Removed != 0 {
		t.Errorf("summary.removed = %d, want 0", s.Removed)
	}
	if s.Modified != 4 {
		t.Errorf("summary.modified = %d, want 4 (App binary + Core binary + Info.plist + CodeResources)", s.Modified)
	}
	if s.Unchanged != 1 {
		t.Errorf("summary.unchanged = %d, want 1 (Core Info.plist)", s.Unchanged)
	}

	// --- Individual nodes ---
	tests := []struct {
		path   string
		status DiffStatus
		kind   FileKind
		isDir  bool
	}{
		{"Payload/App.app/App", Modified, KindMachO, false},
		{"Payload/App.app/Info.plist", Modified, KindPlist, false},
		{"Payload/App.app/_CodeSignature/CodeResources", Modified, KindText, false},
		{"Payload/App.app/Frameworks/Core.framework/Core", Modified, KindMachO, false},
		{"Payload/App.app/Frameworks/Core.framework/Info.plist", Unchanged, KindPlist, false},
		{"Payload/App.app/Frameworks/NewFeature.framework", Added, KindDirectory, true},
		{"Payload/App.app/Frameworks/NewFeature.framework/NewFeature", Added, KindMachO, false},
		{"Payload/App.app/Frameworks/NewFeature.framework/Info.plist", Added, KindPlist, false},
		// Directory status propagation
		{"Payload/App.app", Modified, KindDirectory, true},
		{"Payload/App.app/Frameworks", Modified, KindDirectory, true},
		{"Payload/App.app/Frameworks/Core.framework", Modified, KindDirectory, true},
	}

	for _, tt := range tests {
		n := findNode(result.Root, tt.path)
		if n == nil {
			t.Errorf("node %q not found in tree", tt.path)
			continue
		}
		if n.Status != tt.status {
			t.Errorf("node %q: status = %v, want %v", tt.path, n.Status, tt.status)
		}
		if n.Kind != tt.kind {
			t.Errorf("node %q: kind = %v, want %v", tt.path, n.Kind, tt.kind)
		}
		if n.IsDir != tt.isDir {
			t.Errorf("node %q: is_dir = %v, want %v", tt.path, n.IsDir, tt.isDir)
		}
	}

	// --- Size checks for the main binary ---
	appBin := findNode(result.Root, "Payload/App.app/App")
	if appBin != nil {
		if appBin.SizeA != 16816 {
			t.Errorf("App binary size_a = %d, want 16816", appBin.SizeA)
		}
		if appBin.SizeB != 16864 {
			t.Errorf("App binary size_b = %d, want 16864", appBin.SizeB)
		}
		if appBin.SizeDelta() != 48 {
			t.Errorf("App binary size_delta = %d, want 48", appBin.SizeDelta())
		}
	}

	// --- NewFeature binary should have zero size_a (added) ---
	nf := findNode(result.Root, "Payload/App.app/Frameworks/NewFeature.framework/NewFeature")
	if nf != nil {
		if nf.SizeA != 0 {
			t.Errorf("NewFeature size_a = %d, want 0", nf.SizeA)
		}
		if nf.SizeB != 16840 {
			t.Errorf("NewFeature size_b = %d, want 16840", nf.SizeB)
		}
	}
}

// assertDirSizesAreAggregated checks that every directory node's size equals
// the sum of its children's sizes, not the filesystem metadata size.
func assertDirSizesAreAggregated(t *testing.T, root *Node) {
	t.Helper()
	var check func(*Node)
	check = func(n *Node) {
		if !n.IsDir || len(n.Children) == 0 {
			return
		}
		var childSumA, childSumB int64
		for _, c := range n.Children {
			childSumA += c.SizeA
			childSumB += c.SizeB
		}
		if n.SizeA != childSumA {
			t.Errorf("dir %q: size_a = %d, want %d (sum of children)", n.Path, n.SizeA, childSumA)
		}
		if n.SizeB != childSumB {
			t.Errorf("dir %q: size_b = %d, want %d (sum of children)", n.Path, n.SizeB, childSumB)
		}
		for _, c := range n.Children {
			check(c)
		}
	}
	check(root)
}

func TestIntegration_DirectorySizeAggregation(t *testing.T) {
	td := testdataDir(t)
	pathA := filepath.Join(td, "app-v1")
	pathB := filepath.Join(td, "app-v2")

	result, err := Compare(pathA, pathB, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	assertDirSizesAreAggregated(t, result.Root)
}

func TestIntegration_ArchiveDiff(t *testing.T) {
	td := testdataDir(t)
	pathA := filepath.Join(td, "app-v1.ipa")
	pathB := filepath.Join(td, "app-v2.ipa")

	result, err := Compare(pathA, pathB, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Mode != "tree" {
		t.Fatalf("mode = %q, want tree", result.Mode)
	}

	// The archive diff should produce the same logical result as the directory diff.
	s := result.Summary
	if s.Added != 2 {
		t.Errorf("summary.added = %d, want 2", s.Added)
	}
	if s.Removed != 0 {
		t.Errorf("summary.removed = %d, want 0", s.Removed)
	}
	if s.Modified != 4 {
		t.Errorf("summary.modified = %d, want 4", s.Modified)
	}
	if s.Unchanged != 1 {
		t.Errorf("summary.unchanged = %d, want 1", s.Unchanged)
	}

	// Verify archive correctly detects Mach-O kind inside framework
	core := findNode(result.Root, "Payload/App.app/Frameworks/Core.framework/Core")
	if core == nil {
		t.Fatal("Core binary not found in archive diff tree")
	}
	if core.Kind != KindMachO {
		t.Errorf("Core kind = %v, want macho", core.Kind)
	}
	if core.Status != Modified {
		t.Errorf("Core status = %v, want modified", core.Status)
	}
}

func TestIntegration_AutoDetect_IPA(t *testing.T) {
	td := testdataDir(t)
	pathA := filepath.Join(td, "app-v1.ipa")
	pathB := filepath.Join(td, "app-v2.ipa")

	mode, err := detectMode(pathA, pathB)
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != "tree" {
		t.Errorf("mode = %q, want tree", mode)
	}
}

func TestIntegration_PlistDetail(t *testing.T) {
	td := testdataDir(t)
	pathA := filepath.Join(td, "app-v1")
	pathB := filepath.Join(td, "app-v2")

	result, err := Compare(pathA, pathB, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// App Info.plist: CFBundleVersion 1.0→2.0, CFBundleShortVersionString added
	plist := findNode(result.Root, "Payload/App.app/Info.plist")
	if plist == nil {
		t.Fatal("App Info.plist not found")
	}

	detail, err := Detail(result, plist)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}

	found := make(map[string]DiffStatus)
	values := make(map[string]PlistChange)
	for _, c := range detail.Plist.Changes {
		found[c.KeyPath] = c.Status
		values[c.KeyPath] = c
	}

	if found["CFBundleVersion"] != Modified {
		t.Errorf("CFBundleVersion: %v, want modified", found["CFBundleVersion"])
	}
	if v := values["CFBundleVersion"]; v.ValueA != "1.0" || v.ValueB != "2.0" {
		t.Errorf("CFBundleVersion values = (%q, %q), want (1.0, 2.0)", v.ValueA, v.ValueB)
	}
	if found["CFBundleShortVersionString"] != Added {
		t.Errorf("CFBundleShortVersionString: %v, want added", found["CFBundleShortVersionString"])
	}
	if _, ok := found["CFBundleName"]; ok {
		t.Error("CFBundleName should not appear (unchanged)")
	}

	// Core Info.plist: unchanged - Detail should show no changes.
	corePlist := findNode(result.Root, "Payload/App.app/Frameworks/Core.framework/Info.plist")
	if corePlist == nil {
		t.Fatal("Core Info.plist not found")
	}
	if corePlist.Status != Unchanged {
		t.Fatalf("Core Info.plist status = %v, want unchanged", corePlist.Status)
	}
}

func TestIntegration_PlistDetail_Archive(t *testing.T) {
	td := testdataDir(t)
	pathA := filepath.Join(td, "app-v1.ipa")
	pathB := filepath.Join(td, "app-v2.ipa")

	result, err := Compare(pathA, pathB, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	plist := findNode(result.Root, "Payload/App.app/Info.plist")
	if plist == nil {
		t.Fatal("App Info.plist not found in archive diff")
	}

	detail, err := Detail(result, plist)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}

	found := make(map[string]DiffStatus)
	for _, c := range detail.Plist.Changes {
		found[c.KeyPath] = c.Status
	}

	if found["CFBundleVersion"] != Modified {
		t.Errorf("CFBundleVersion: %v, want modified", found["CFBundleVersion"])
	}
	if found["CFBundleShortVersionString"] != Added {
		t.Errorf("CFBundleShortVersionString: %v, want added", found["CFBundleShortVersionString"])
	}
}

func TestIntegration_MixedInputs(t *testing.T) {
	td := testdataDir(t)
	// Compare a directory against an IPA - should work as tree mode.
	pathA := filepath.Join(td, "app-v1")
	pathB := filepath.Join(td, "app-v2.ipa")

	result, err := Compare(pathA, pathB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// Should still produce a valid diff.
	if result.Summary.Added+result.Summary.Removed+result.Summary.Modified == 0 {
		t.Error("expected some changes in mixed dir/archive diff")
	}
}
