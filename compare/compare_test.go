package compare

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareDirectories_AddedRemovedModified(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeFile(t, filepath.Join(dirA, "same.txt"), "hello")
	writeFile(t, filepath.Join(dirB, "same.txt"), "hello")

	writeFile(t, filepath.Join(dirA, "changed.txt"), "short")
	writeFile(t, filepath.Join(dirB, "changed.txt"), "this is longer content")

	writeFile(t, filepath.Join(dirA, "removed.txt"), "gone")

	writeFile(t, filepath.Join(dirB, "added.txt"), "new")

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.Mode != "tree" {
		t.Errorf("mode = %q, want tree", result.Mode)
	}

	s := result.Summary
	if s.Added != 1 {
		t.Errorf("added = %d, want 1", s.Added)
	}
	if s.Removed != 1 {
		t.Errorf("removed = %d, want 1", s.Removed)
	}
	if s.Modified != 1 {
		t.Errorf("modified = %d, want 1", s.Modified)
	}
	if s.Unchanged != 1 {
		t.Errorf("unchanged = %d, want 1", s.Unchanged)
	}
}

func TestCompareDirectories_Nested(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.MkdirAll(filepath.Join(dirA, "sub", "deep"), 0o755)
	os.MkdirAll(filepath.Join(dirB, "sub", "deep"), 0o755)

	writeFile(t, filepath.Join(dirA, "sub", "deep", "file.txt"), "a")
	writeFile(t, filepath.Join(dirB, "sub", "deep", "file.txt"), "bb")

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Summary.Modified != 1 {
		t.Errorf("modified = %d, want 1", result.Summary.Modified)
	}
	// Root should be Modified because a child changed.
	if result.Root.Status != Modified {
		t.Errorf("root status = %v, want modified", result.Root.Status)
	}
}

func TestCompareArchives(t *testing.T) {
	dir := t.TempDir()
	zipA := filepath.Join(dir, "a.zip")
	zipB := filepath.Join(dir, "b.zip")

	createZip(t, zipA, map[string]string{
		"file.txt":       "hello",
		"removed.txt":    "gone",
		"sub/nested.txt": "content",
	})
	createZip(t, zipB, map[string]string{
		"file.txt":       "hello world",
		"added.txt":      "new",
		"sub/nested.txt": "content",
	})

	result, err := Compare(zipA, zipB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	s := result.Summary
	if s.Added != 1 {
		t.Errorf("added = %d, want 1", s.Added)
	}
	if s.Removed != 1 {
		t.Errorf("removed = %d, want 1", s.Removed)
	}
	if s.Modified != 1 {
		t.Errorf("modified = %d, want 1", s.Modified)
	}
}

func TestDetectMode_Directories(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	mode, err := detectMode(dirA, dirB)
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != "tree" {
		t.Errorf("mode = %q, want tree", mode)
	}
}

func TestDetectMode_Archives(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.ipa")
	b := filepath.Join(dir, "b.ipa")
	createZip(t, a, map[string]string{"f": "x"})
	createZip(t, b, map[string]string{"f": "y"})

	mode, err := detectMode(a, b)
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != "tree" {
		t.Errorf("mode = %q, want tree", mode)
	}
}

func TestDetectMode_Plists(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.plist")
	b := filepath.Join(dir, "b.plist")
	writeFile(t, a, "<plist></plist>")
	writeFile(t, b, "<plist></plist>")

	mode, err := detectMode(a, b)
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != "plist" {
		t.Errorf("mode = %q, want plist", mode)
	}
}

func TestDetectMode_TextFallback(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.swift")
	b := filepath.Join(dir, "b.swift")
	writeFile(t, a, "import Foundation")
	writeFile(t, b, "import UIKit")

	mode, err := detectMode(a, b)
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != "text" {
		t.Errorf("mode = %q, want text", mode)
	}
}

func TestClassifyPath(t *testing.T) {
	tests := []struct {
		path  string
		isDir bool
		want  FileKind
	}{
		{"App.app/Info.plist", false, KindPlist},
		{"lib/libFoo.dylib", false, KindMachO},
		{"Foo.framework/Foo", false, KindMachO},
		{"App.app/App", false, KindMachO},
		{"App.app/_CodeSignature/CodeResources", false, KindText},
		{"Foo.framework", true, KindDirectory},
		{"Foo.app.dSYM", true, KindDSYM},
		{"README.md", false, KindText},
	}

	for _, tt := range tests {
		got := classifyPath(tt.path, tt.isDir)
		if got != tt.want {
			t.Errorf("classifyPath(%q, %v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
		}
	}
}

func TestComputeSummary(t *testing.T) {
	root := &Node{
		IsDir: true,
		Children: []*Node{
			{Status: Added, SizeB: 100},
			{Status: Removed, SizeA: 50},
			{Status: Modified, SizeA: 200, SizeB: 250},
			{Status: Unchanged, SizeA: 300, SizeB: 300},
		},
	}

	s := ComputeSummary(root)
	if s.Added != 1 || s.Removed != 1 || s.Modified != 1 || s.Unchanged != 1 {
		t.Errorf("counts wrong: %+v", s)
	}
	// Delta: (100-0) + (0-50) + (250-200) + (300-300) = 100
	if s.SizeDelta != 100 {
		t.Errorf("size_delta = %d, want 100", s.SizeDelta)
	}
}

func TestSameSizeDifferentContent(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Same size (4 bytes each) but different content.
	writeFile(t, filepath.Join(dirA, "file.txt"), "aaaa")
	writeFile(t, filepath.Join(dirB, "file.txt"), "bbbb")

	// Same size AND same content → should stay unchanged.
	writeFile(t, filepath.Join(dirA, "identical.txt"), "same")
	writeFile(t, filepath.Join(dirB, "identical.txt"), "same")

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	s := result.Summary
	if s.Modified != 1 {
		t.Errorf("modified = %d, want 1 (file.txt same size, different content)", s.Modified)
	}
	if s.Unchanged != 1 {
		t.Errorf("unchanged = %d, want 1 (identical.txt)", s.Unchanged)
	}
}

func TestSameSizeDifferentContent_Archive(t *testing.T) {
	dir := t.TempDir()
	zipA := filepath.Join(dir, "a.zip")
	zipB := filepath.Join(dir, "b.zip")

	createZip(t, zipA, map[string]string{"file.txt": "aaaa"})
	createZip(t, zipB, map[string]string{"file.txt": "bbbb"})

	result, err := Compare(zipA, zipB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Summary.Modified != 1 {
		t.Errorf("modified = %d, want 1 (same size, different content in archive)", result.Summary.Modified)
	}
}

func TestDirectorySizeAggregation(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	os.MkdirAll(filepath.Join(dirA, "sub"), 0o755)
	os.MkdirAll(filepath.Join(dirB, "sub"), 0o755)

	writeFile(t, filepath.Join(dirA, "sub", "file1.txt"), strings.Repeat("a", 100))
	writeFile(t, filepath.Join(dirA, "sub", "file2.txt"), strings.Repeat("b", 200))
	writeFile(t, filepath.Join(dirB, "sub", "file1.txt"), strings.Repeat("a", 150))
	writeFile(t, filepath.Join(dirB, "sub", "file2.txt"), strings.Repeat("b", 200))

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	sub := findNode(result.Root, "sub")
	if sub == nil {
		t.Fatal("sub directory not found")
	}

	// Directory size must be the sum of children's file sizes, not the
	// filesystem directory entry metadata size.
	if sub.SizeA != 300 {
		t.Errorf("sub dir size_a = %d, want 300 (sum of children)", sub.SizeA)
	}
	if sub.SizeB != 350 {
		t.Errorf("sub dir size_b = %d, want 350 (sum of children)", sub.SizeB)
	}
}

func TestArchiveFormatFor(t *testing.T) {
	tests := []struct {
		path string
		want archiveFormat
	}{
		{"app.zip", archiveZip},
		{"app.ipa", archiveZip},
		{"app.jar", archiveZip},
		{"app.apk", archiveZip},
		{"lib.aar", archiveZip},
		{"data.tar", archiveTar},
		{"data.tar.gz", archiveTarGz},
		{"data.tgz", archiveTarGz},
		{"data.tar.bz2", archiveTarBz2},
		{"data.tbz2", archiveTarBz2},
		{"DATA.TAR.GZ", archiveTarGz},
		{"file.txt", archiveNone},
		{"file.gz", archiveNone},
		{"file.bz2", archiveNone},
	}
	for _, tt := range tests {
		got := archiveFormatFor(tt.path)
		if got != tt.want {
			t.Errorf("archiveFormatFor(%q) = %d, want %d", tt.path, got, tt.want)
		}
	}
}

func TestCompareTarArchives(t *testing.T) {
	dir := t.TempDir()
	tarA := filepath.Join(dir, "a.tar")
	tarB := filepath.Join(dir, "b.tar")

	createTar(t, tarA, map[string]string{
		"file.txt":       "hello",
		"removed.txt":    "gone",
		"sub/nested.txt": "content",
	})
	createTar(t, tarB, map[string]string{
		"file.txt":       "hello world",
		"added.txt":      "new",
		"sub/nested.txt": "content",
	})

	result, err := Compare(tarA, tarB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	s := result.Summary
	if s.Added != 1 {
		t.Errorf("added = %d, want 1", s.Added)
	}
	if s.Removed != 1 {
		t.Errorf("removed = %d, want 1", s.Removed)
	}
	if s.Modified != 1 {
		t.Errorf("modified = %d, want 1", s.Modified)
	}
}

func TestCompareTarGzArchives(t *testing.T) {
	dir := t.TempDir()
	tarA := filepath.Join(dir, "a.tar.gz")
	tarB := filepath.Join(dir, "b.tar.gz")

	createTarGz(t, tarA, map[string]string{
		"file.txt": "hello",
		"only.txt": "a-only",
	})
	createTarGz(t, tarB, map[string]string{
		"file.txt": "world",
		"new.txt":  "b-only",
	})

	result, err := Compare(tarA, tarB, "")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if result.Mode != "tree" {
		t.Errorf("mode = %q, want tree", result.Mode)
	}

	s := result.Summary
	if s.Added != 1 {
		t.Errorf("added = %d, want 1", s.Added)
	}
	if s.Removed != 1 {
		t.Errorf("removed = %d, want 1", s.Removed)
	}
	if s.Modified != 1 {
		t.Errorf("modified = %d, want 1", s.Modified)
	}
}

func TestDetectMode_TarArchives(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.tar.gz")
	b := filepath.Join(dir, "b.tgz")
	createTarGz(t, a, map[string]string{"f": "x"})
	createTarGz(t, b, map[string]string{"f": "y"})

	mode, err := detectMode(a, b)
	if err != nil {
		t.Fatalf("detectMode: %v", err)
	}
	if mode != "tree" {
		t.Errorf("mode = %q, want tree", mode)
	}
}

func TestClassifyPath_Archives(t *testing.T) {
	tests := []struct {
		path string
		want FileKind
	}{
		{"lib.jar", KindArchive},
		{"app.apk", KindArchive},
		{"data.tar", KindArchive},
		{"data.tar.gz", KindArchive},
		{"data.tgz", KindArchive},
	}
	for _, tt := range tests {
		got := classifyPath(tt.path, false)
		if got != tt.want {
			t.Errorf("classifyPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestDirWithArchiveExpansion(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Both dirs have a zip with overlapping contents.
	createZip(t, filepath.Join(dirA, "bundle.zip"), map[string]string{
		"file.txt":       "hello",
		"removed.txt":    "gone",
		"sub/nested.txt": "content",
	})
	createZip(t, filepath.Join(dirB, "bundle.zip"), map[string]string{
		"file.txt":       "hello world",
		"added.txt":      "new",
		"sub/nested.txt": "content",
	})

	// Also a regular file alongside.
	writeFile(t, filepath.Join(dirA, "readme.txt"), "v1")
	writeFile(t, filepath.Join(dirB, "readme.txt"), "v2")

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// bundle.zip should be expanded — its children should show up.
	bundle := findNode(result.Root, "bundle.zip")
	if bundle == nil {
		t.Fatal("bundle.zip not found in tree")
	}
	if !bundle.IsDir {
		t.Error("bundle.zip should be treated as a directory")
	}
	if len(bundle.Children) == 0 {
		t.Error("bundle.zip should have expanded children")
	}

	// Verify the summary includes changes from inside the archive.
	// Inside archive: 1 modified (file.txt), 1 added (added.txt), 1 removed (removed.txt), 1 unchanged (sub/nested.txt)
	// Plus: 1 modified (readme.txt)
	s := result.Summary
	if s.Added != 1 {
		t.Errorf("added = %d, want 1", s.Added)
	}
	if s.Removed != 1 {
		t.Errorf("removed = %d, want 1", s.Removed)
	}
	if s.Modified != 2 { // file.txt inside zip + readme.txt
		t.Errorf("modified = %d, want 2", s.Modified)
	}
}

func TestDirWithTarGzExpansion(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	createTarGz(t, filepath.Join(dirA, "data.tar.gz"), map[string]string{
		"a.txt": "one",
	})
	createTarGz(t, filepath.Join(dirB, "data.tar.gz"), map[string]string{
		"a.txt": "two",
	})

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	node := findNode(result.Root, "data.tar.gz")
	if node == nil {
		t.Fatal("data.tar.gz not found")
	}
	if !node.IsDir {
		t.Error("tar.gz should be treated as directory")
	}
	if result.Summary.Modified != 1 {
		t.Errorf("modified = %d, want 1", result.Summary.Modified)
	}
}

func TestDirWithArchive_ContentHash(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	// Same-size but different content — must detect via hash.
	createZip(t, filepath.Join(dirA, "app.zip"), map[string]string{
		"data.txt": "aaaa",
	})
	createZip(t, filepath.Join(dirB, "app.zip"), map[string]string{
		"data.txt": "bbbb",
	})

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if result.Summary.Modified != 1 {
		t.Errorf("modified = %d, want 1 (same size, different content through archive)", result.Summary.Modified)
	}
}

func TestDirWithArchive_Detail(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	createZip(t, filepath.Join(dirA, "app.zip"), map[string]string{
		"hello.txt": "line one\nline two\n",
	})
	createZip(t, filepath.Join(dirB, "app.zip"), map[string]string{
		"hello.txt": "line one\nline three\n",
	})

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	// Find the inner text file.
	appZip := findNode(result.Root, "app.zip")
	if appZip == nil {
		t.Fatal("app.zip not found")
	}
	var textNode *Node
	for _, c := range appZip.Children {
		if c.Name == "hello.txt" {
			textNode = c
			break
		}
	}
	if textNode == nil {
		t.Fatal("hello.txt not found inside app.zip")
	}

	// Detail should work — reads content through the archive boundary.
	detail, err := Detail(result, textNode)
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

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func createTar(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o644,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
}

func createTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o644,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}
