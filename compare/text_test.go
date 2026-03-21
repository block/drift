package compare

import (
	"path/filepath"
	"testing"
)

func TestCompareText_Modified(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeFile(t, filepath.Join(dirA, "file.swift"), "import Foundation\nlet x = 1\n")
	writeFile(t, filepath.Join(dirB, "file.swift"), "import Foundation\nlet x = 2\nlet y = 3\n")

	diff, err := compareText(dirA, dirB, "file.swift", Modified)
	if err != nil {
		t.Fatalf("compareText: %v", err)
	}

	if len(diff.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	// Should have both added and removed lines.
	var hasAdded, hasRemoved bool
	for _, h := range diff.Hunks {
		for _, l := range h.Lines {
			if l.Kind == "added" {
				hasAdded = true
			}
			if l.Kind == "removed" {
				hasRemoved = true
			}
		}
	}
	if !hasAdded || !hasRemoved {
		t.Errorf("expected added and removed lines, got added=%v removed=%v", hasAdded, hasRemoved)
	}
}

func TestCompareText_Identical(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeFile(t, filepath.Join(dirA, "file.txt"), "same content\n")
	writeFile(t, filepath.Join(dirB, "file.txt"), "same content\n")

	diff, err := compareText(dirA, dirB, "file.txt", Unchanged)
	if err != nil {
		t.Fatalf("compareText: %v", err)
	}

	if len(diff.Hunks) != 0 {
		t.Errorf("expected no hunks for identical files, got %d", len(diff.Hunks))
	}
}

func TestCompareText_Added(t *testing.T) {
	dirB := t.TempDir()
	writeFile(t, filepath.Join(dirB, "new.txt"), "line 1\nline 2\n")

	diff, err := compareText("", dirB, "new.txt", Added)
	if err != nil {
		t.Fatalf("compareText: %v", err)
	}

	if len(diff.Hunks) == 0 {
		t.Fatal("expected hunks for added file")
	}

	for _, h := range diff.Hunks {
		for _, l := range h.Lines {
			if l.Kind == "removed" {
				t.Error("added file should have no removed lines")
			}
		}
	}
}

func TestCompareText_Removed(t *testing.T) {
	dirA := t.TempDir()
	writeFile(t, filepath.Join(dirA, "old.txt"), "line 1\nline 2\n")

	diff, err := compareText(dirA, "", "old.txt", Removed)
	if err != nil {
		t.Fatalf("compareText: %v", err)
	}

	if len(diff.Hunks) == 0 {
		t.Fatal("expected hunks for removed file")
	}

	for _, h := range diff.Hunks {
		for _, l := range h.Lines {
			if l.Kind == "added" {
				t.Error("removed file should have no added lines")
			}
		}
	}
}

func TestCompareText_Standalone(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.swift")
	b := filepath.Join(dir, "b.swift")
	writeFile(t, a, "import Foundation\n")
	writeFile(t, b, "import UIKit\n")

	result, err := Compare(a, b, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Mode != "text" {
		t.Errorf("mode = %q, want text", result.Mode)
	}
	if result.Root.Status != Modified {
		t.Errorf("status = %v, want modified", result.Root.Status)
	}
	if result.Root.Kind != KindText {
		t.Errorf("kind = %v, want text", result.Root.Kind)
	}
}

func TestCompareText_StandaloneIdentical(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a, "same\n")
	writeFile(t, b, "same\n")

	result, err := Compare(a, b, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Root.Status != Unchanged {
		t.Errorf("status = %v, want unchanged", result.Root.Status)
	}
}

func TestParseUnifiedDiff(t *testing.T) {
	input := `@@ -1,3 +1,4 @@
 line 1
-line 2
+line 2 modified
+line 2b
 line 3
`
	hunks, err := parseUnifiedDiff(input)
	if err != nil {
		t.Fatalf("parseUnifiedDiff: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("got %d hunks, want 1", len(hunks))
	}

	h := hunks[0]
	if h.OldStart != 1 || h.OldCount != 3 || h.NewStart != 1 || h.NewCount != 4 {
		t.Errorf("hunk header = -%d,%d +%d,%d, want -1,3 +1,4",
			h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	}
	if len(h.Lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(h.Lines))
	}
	if h.Lines[0].Kind != "context" {
		t.Errorf("line 0 kind = %q, want context", h.Lines[0].Kind)
	}
	if h.Lines[1].Kind != "removed" {
		t.Errorf("line 1 kind = %q, want removed", h.Lines[1].Kind)
	}
	if h.Lines[2].Kind != "added" {
		t.Errorf("line 2 kind = %q, want added", h.Lines[2].Kind)
	}
}

func TestDetailText(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeFile(t, filepath.Join(dirA, "config.txt"), "key=old\n")
	writeFile(t, filepath.Join(dirB, "config.txt"), "key=new\n")

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	node := findNodeInTree(result.Root, "config.txt")
	if node == nil {
		t.Fatal("config.txt not found in tree")
	}
	if node.Kind != KindText {
		t.Fatalf("kind = %v, want text", node.Kind)
	}

	detail, err := Detail(result, node)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Text == nil {
		t.Fatal("expected text diff")
	}
	if len(detail.Text.Hunks) == 0 {
		t.Error("expected at least one hunk")
	}
}

func TestDetailText_Standalone(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "old.swift")
	b := filepath.Join(dir, "new.swift")
	writeFile(t, a, "import Foundation\nlet x = 1\n")
	writeFile(t, b, "import UIKit\nlet x = 2\nlet y = 3\n")

	result, err := Compare(a, b, "text")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	detail, err := Detail(result, result.Root)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Text == nil {
		t.Fatal("expected text diff")
	}
	if len(detail.Text.Hunks) == 0 {
		t.Error("expected at least one hunk")
	}

	// Verify we got meaningful changes.
	var added, removed int
	for _, h := range detail.Text.Hunks {
		for _, l := range h.Lines {
			switch l.Kind {
			case "added":
				added++
			case "removed":
				removed++
			}
		}
	}
	if added == 0 || removed == 0 {
		t.Errorf("expected both added and removed lines, got added=%d removed=%d", added, removed)
	}
}

func TestDetailText_StandaloneDifferentNames(t *testing.T) {
	// Files in different directories with different names.
	dirA := t.TempDir()
	dirB := t.TempDir()
	a := filepath.Join(dirA, "alpha.txt")
	b := filepath.Join(dirB, "beta.txt")
	writeFile(t, a, "line 1\nline 2\n")
	writeFile(t, b, "line 1\nline 2 changed\nline 3\n")

	result, err := Compare(a, b, "text")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	detail, err := Detail(result, result.Root)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Text == nil {
		t.Fatal("expected text diff")
	}
	if len(detail.Text.Hunks) == 0 {
		t.Error("expected at least one hunk")
	}
}

// findNodeInTree searches the tree for a node by name (breadth-first).
func findNodeInTree(root *Node, name string) *Node {
	if root.Name == name {
		return root
	}
	for _, c := range root.Children {
		if found := findNodeInTree(c, name); found != nil {
			return found
		}
	}
	return nil
}
