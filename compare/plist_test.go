package compare

import (
	"testing"
)

func TestParsePlistXML(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
    <key>Name</key>
    <string>App</string>
    <key>Version</key>
    <integer>42</integer>
    <key>Debug</key>
    <true/>
</dict>
</plist>`)

	val, err := parsePlistXML(data)
	if err != nil {
		t.Fatalf("parsePlistXML: %v", err)
	}

	dict, ok := val.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", val)
	}
	if dict["Name"] != "App" {
		t.Errorf("Name = %v, want App", dict["Name"])
	}
	if dict["Version"] != "42" {
		t.Errorf("Version = %v, want 42", dict["Version"])
	}
	if dict["Debug"] != true {
		t.Errorf("Debug = %v, want true", dict["Debug"])
	}
}

func TestParsePlistXML_Nested(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
    <key>Outer</key>
    <dict>
        <key>Inner</key>
        <string>value</string>
    </dict>
    <key>Items</key>
    <array>
        <string>a</string>
        <string>b</string>
    </array>
</dict>
</plist>`)

	val, err := parsePlistXML(data)
	if err != nil {
		t.Fatalf("parsePlistXML: %v", err)
	}

	dict := val.(map[string]any)
	outer := dict["Outer"].(map[string]any)
	if outer["Inner"] != "value" {
		t.Errorf("Outer.Inner = %v, want value", outer["Inner"])
	}

	items := dict["Items"].([]any)
	if len(items) != 2 || items[0] != "a" || items[1] != "b" {
		t.Errorf("Items = %v, want [a b]", items)
	}
}

func TestDiffPlistValues_Modified(t *testing.T) {
	a := map[string]any{
		"CFBundleVersion": "1.0",
		"CFBundleName":    "App",
	}
	b := map[string]any{
		"CFBundleVersion":            "2.0",
		"CFBundleName":               "App",
		"CFBundleShortVersionString": "2.0.0",
	}

	changes := diffPlistValues("", a, b)

	want := map[string]DiffStatus{
		"CFBundleName":               Unchanged, // should NOT appear (unchanged)
		"CFBundleShortVersionString": Added,
		"CFBundleVersion":            Modified,
	}

	found := make(map[string]DiffStatus)
	for _, c := range changes {
		found[c.KeyPath] = c.Status
	}

	for key, status := range want {
		if status == Unchanged {
			if _, ok := found[key]; ok {
				t.Errorf("key %q should not appear in changes (unchanged)", key)
			}
			continue
		}
		if found[key] != status {
			t.Errorf("key %q: status = %v, want %v", key, found[key], status)
		}
	}

	// Check values on the modified key.
	for _, c := range changes {
		if c.KeyPath == "CFBundleVersion" {
			if c.ValueA != "1.0" || c.ValueB != "2.0" {
				t.Errorf("CFBundleVersion values = (%q, %q), want (1.0, 2.0)", c.ValueA, c.ValueB)
			}
		}
	}
}

func TestDiffPlistValues_AddedRemoved(t *testing.T) {
	a := map[string]any{"removed": "old"}
	b := map[string]any{"added": "new"}

	changes := diffPlistValues("", a, b)

	found := make(map[string]DiffStatus)
	for _, c := range changes {
		found[c.KeyPath] = c.Status
	}

	if found["removed"] != Removed {
		t.Errorf("removed: status = %v, want removed", found["removed"])
	}
	if found["added"] != Added {
		t.Errorf("added: status = %v, want added", found["added"])
	}
}

func TestDiffPlistValues_NestedDict(t *testing.T) {
	a := map[string]any{
		"NSAppTransportSecurity": map[string]any{
			"NSAllowsArbitraryLoads": true,
		},
	}
	b := map[string]any{
		"NSAppTransportSecurity": map[string]any{
			"NSAllowsArbitraryLoads": false,
		},
	}

	changes := diffPlistValues("", a, b)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	c := changes[0]
	if c.KeyPath != "NSAppTransportSecurity.NSAllowsArbitraryLoads" {
		t.Errorf("key_path = %q, want NSAppTransportSecurity.NSAllowsArbitraryLoads", c.KeyPath)
	}
	if c.Status != Modified {
		t.Errorf("status = %v, want modified", c.Status)
	}
}

func TestDiffPlistValues_Array(t *testing.T) {
	a := map[string]any{
		"items": []any{"a", "b"},
	}
	b := map[string]any{
		"items": []any{"a", "c", "d"},
	}

	changes := diffPlistValues("", a, b)

	found := make(map[string]DiffStatus)
	for _, c := range changes {
		found[c.KeyPath] = c.Status
	}

	// items[0] unchanged (not in changes), items[1] modified, items[2] added
	if _, ok := found["items[0]"]; ok {
		t.Error("items[0] should not appear (unchanged)")
	}
	if found["items[1]"] != Modified {
		t.Errorf("items[1]: status = %v, want modified", found["items[1]"])
	}
	if found["items[2]"] != Added {
		t.Errorf("items[2]: status = %v, want added", found["items[2]"])
	}
}

func TestDiffPlistValues_Identical(t *testing.T) {
	a := map[string]any{"key": "same"}
	b := map[string]any{"key": "same"}

	changes := diffPlistValues("", a, b)
	if len(changes) != 0 {
		t.Errorf("expected no changes for identical plists, got %d", len(changes))
	}
}

func TestFormatPlistValue(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{"hello", "hello"},
		{true, "true"},
		{false, "false"},
		{map[string]any{"a": 1, "b": 2}, "<dict: 2 keys>"},
		{[]any{"x", "y", "z"}, "<array: 3 items>"},
		{nil, ""},
	}

	for _, tt := range tests {
		got := formatPlistValue(tt.input)
		if got != tt.want {
			t.Errorf("formatPlistValue(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDetail_PlistDiff(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	writeFile(t, dirA+"/Info.plist", `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
    <key>CFBundleVersion</key>
    <string>1.0</string>
    <key>CFBundleName</key>
    <string>App</string>
</dict>
</plist>`)

	writeFile(t, dirB+"/Info.plist", `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
    <key>CFBundleVersion</key>
    <string>2.0</string>
    <key>CFBundleName</key>
    <string>App</string>
    <key>NewKey</key>
    <string>value</string>
</dict>
</plist>`)

	result, err := Compare(dirA, dirB, "tree")
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	plistNode := findNode(result.Root, "Info.plist")
	if plistNode == nil {
		t.Fatal("Info.plist node not found")
	}
	if plistNode.Status != Modified {
		t.Fatalf("Info.plist status = %v, want modified", plistNode.Status)
	}

	detail, err := Detail(result, plistNode)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}

	if detail.Kind != KindPlist {
		t.Errorf("detail kind = %v, want plist", detail.Kind)
	}
	if detail.Plist == nil {
		t.Fatal("detail.Plist is nil")
	}

	found := make(map[string]DiffStatus)
	for _, c := range detail.Plist.Changes {
		found[c.KeyPath] = c.Status
	}

	if found["CFBundleVersion"] != Modified {
		t.Errorf("CFBundleVersion: %v, want modified", found["CFBundleVersion"])
	}
	if found["NewKey"] != Added {
		t.Errorf("NewKey: %v, want added", found["NewKey"])
	}
	if _, ok := found["CFBundleName"]; ok {
		t.Error("CFBundleName should not appear (unchanged)")
	}
}

func TestDetail_UnsupportedKind(t *testing.T) {
	result := &Result{PathA: "/a", PathB: "/b"}
	node := &Node{Kind: KindMachO, Status: Modified}

	_, err := Detail(result, node)
	if err == nil {
		t.Error("expected error for unsupported kind")
	}
}
