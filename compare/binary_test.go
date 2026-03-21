package compare

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseNmOutput(t *testing.T) {
	output := `_main
_helper_function
_global_data
__swift_FORCE_LOAD_$_swiftFoundation
`
	syms := parseNmOutput(output)

	want := []string{
		"_main",
		"_helper_function",
		"_global_data",
		"__swift_FORCE_LOAD_$_swiftFoundation",
	}
	for _, name := range want {
		if !syms[name] {
			t.Errorf("symbol %q not found", name)
		}
	}
	if len(syms) != len(want) {
		t.Errorf("got %d symbols, want %d", len(syms), len(want))
	}
}

func TestParseNmOutput_Empty(t *testing.T) {
	syms := parseNmOutput("")
	if len(syms) != 0 {
		t.Errorf("got %d symbols for empty output, want 0", len(syms))
	}
}

func TestParseSizeOutput(t *testing.T) {
	output := `Segment __PAGEZERO: 4294967296
Segment __TEXT: 32768
	Section __text: 16384
	Section __stubs: 120
	Section __stub_helper: 200
	Section __cstring: 15
	Section __unwind_info: 72
Segment __DATA_CONST: 16384
	Section __got: 48
Segment __LINKEDIT: 16384
total 4295032832`

	sections := parseSizeOutput(output)

	tests := []struct {
		segment string
		section string
		size    int64
	}{
		{"__TEXT", "__text", 16384},
		{"__TEXT", "__stubs", 120},
		{"__TEXT", "__stub_helper", 200},
		{"__TEXT", "__cstring", 15},
		{"__TEXT", "__unwind_info", 72},
		{"__DATA_CONST", "__got", 48},
	}

	if len(sections) != len(tests) {
		t.Fatalf("got %d sections, want %d", len(sections), len(tests))
	}
	for i, tt := range tests {
		s := sections[i]
		if s.segment != tt.segment || s.section != tt.section || s.size != tt.size {
			t.Errorf("section[%d] = {%s, %s, %d}, want {%s, %s, %d}",
				i, s.segment, s.section, s.size, tt.segment, tt.section, tt.size)
		}
	}
}

func TestDiffSymbols(t *testing.T) {
	symsA := map[string]bool{
		"_main":    true,
		"_shared":  true,
		"_removed": true,
	}
	symsB := map[string]bool{
		"_main":   true,
		"_shared": true,
		"_added":  true,
	}

	changes := diffSymbols(symsA, symsB)

	found := make(map[string]DiffStatus)
	for _, c := range changes {
		found[c.Name] = c.Status
	}

	if found["_removed"] != Removed {
		t.Errorf("_removed: %v, want removed", found["_removed"])
	}
	if found["_added"] != Added {
		t.Errorf("_added: %v, want added", found["_added"])
	}
	if _, ok := found["_main"]; ok {
		t.Error("_main should not appear (unchanged)")
	}
	if _, ok := found["_shared"]; ok {
		t.Error("_shared should not appear (unchanged)")
	}
	if len(changes) != 2 {
		t.Errorf("got %d changes, want 2", len(changes))
	}
}

func TestDiffSymbols_NilMaps(t *testing.T) {
	// Added binary: all symbols are new.
	changes := diffSymbols(nil, map[string]bool{"_main": true, "_foo": true})
	if len(changes) != 2 {
		t.Errorf("got %d changes, want 2", len(changes))
	}
	for _, c := range changes {
		if c.Status != Added {
			t.Errorf("symbol %q: %v, want added", c.Name, c.Status)
		}
	}
}

func TestDiffSections(t *testing.T) {
	secsA := []sectionInfo{
		{"__TEXT", "__text", 16384},
		{"__TEXT", "__stubs", 120},
		{"__DATA", "__data", 1024},
	}
	secsB := []sectionInfo{
		{"__TEXT", "__text", 20480},
		{"__TEXT", "__stubs", 120},
		{"__TEXT", "__cstring", 256},
	}

	changes := diffSections(secsA, secsB)

	found := make(map[string]SectionChange)
	for _, c := range changes {
		key := c.Segment + "." + c.Section
		found[key] = c
	}

	// __text grew
	if c, ok := found["__TEXT.__text"]; !ok || c.SizeA != 16384 || c.SizeB != 20480 {
		t.Errorf("__TEXT.__text: got %+v, want 16384→20480", found["__TEXT.__text"])
	}
	// __stubs unchanged — should NOT appear
	if _, ok := found["__TEXT.__stubs"]; ok {
		t.Error("__TEXT.__stubs should not appear (unchanged)")
	}
	// __data removed (only in A)
	if c, ok := found["__DATA.__data"]; !ok || c.SizeA != 1024 || c.SizeB != 0 {
		t.Errorf("__DATA.__data: got %+v, want 1024→0", found["__DATA.__data"])
	}
	// __cstring added (only in B)
	if c, ok := found["__TEXT.__cstring"]; !ok || c.SizeA != 0 || c.SizeB != 256 {
		t.Errorf("__TEXT.__cstring: got %+v, want 0→256", found["__TEXT.__cstring"])
	}
}

// TestCompareBinary_Compiled compiles two small C programs and diffs them.
func TestCompareBinary_Compiled(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler not available")
	}

	dir := t.TempDir()
	dirA := filepath.Join(dir, "a")
	dirB := filepath.Join(dir, "b")
	os.MkdirAll(dirA, 0o755)
	os.MkdirAll(dirB, 0o755)

	writeFile(t, filepath.Join(dir, "a.c"),
		"void shared_func(void) {}\nvoid removed_func(void) {}\nint main(void) { return 0; }\n")
	writeFile(t, filepath.Join(dir, "b.c"),
		"void shared_func(void) {}\nvoid added_func(void) {}\nint main(void) { return 0; }\n")

	if err := exec.Command("cc", "-o", filepath.Join(dirA, "prog"), filepath.Join(dir, "a.c")).Run(); err != nil {
		t.Fatalf("compiling a.c: %v", err)
	}
	if err := exec.Command("cc", "-o", filepath.Join(dirB, "prog"), filepath.Join(dir, "b.c")).Run(); err != nil {
		t.Fatalf("compiling b.c: %v", err)
	}

	diff, err := compareBinary(dirA, dirB, "prog", Modified)
	if err != nil {
		t.Fatalf("compareBinary: %v", err)
	}

	// Check symbol changes.
	symStatus := make(map[string]DiffStatus)
	for _, c := range diff.Symbols {
		symStatus[c.Name] = c.Status
	}

	if symStatus["_removed_func"] != Removed {
		t.Errorf("_removed_func: %v, want removed", symStatus["_removed_func"])
	}
	if symStatus["_added_func"] != Added {
		t.Errorf("_added_func: %v, want added", symStatus["_added_func"])
	}
	if _, ok := symStatus["_main"]; ok {
		t.Error("_main should not appear (unchanged)")
	}
	if _, ok := symStatus["_shared_func"]; ok {
		t.Error("_shared_func should not appear (unchanged)")
	}

	// Sections should be present (at minimum __TEXT.__text).
	if len(diff.Sections) == 0 {
		// Sections might be identical if compiler produces same layout.
		// Don't fail, just note.
		t.Log("no section changes (binaries may have identical layout)")
	}
}
