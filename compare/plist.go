package compare

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// PlistDiff contains key-level changes between two plists.
type PlistDiff struct {
	Changes []PlistChange `json:"changes"`
}

// PlistChange represents a single key-path change in a plist diff.
type PlistChange struct {
	KeyPath string     `json:"key_path"`
	Status  DiffStatus `json:"status"`
	ValueA  string     `json:"value_a,omitempty"`
	ValueB  string     `json:"value_b,omitempty"`
}

// comparePlist reads two plists from their sources and produces a structured diff.
func comparePlist(sourceA, sourceB, relPath string, status DiffStatus) (*PlistDiff, error) {
	var dataA, dataB []byte
	var err error

	if status != Added {
		dataA, err = readContent(sourceA, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading plist A: %w", err)
		}
	}
	if status != Removed {
		dataB, err = readContent(sourceB, relPath)
		if err != nil {
			return nil, fmt.Errorf("reading plist B: %w", err)
		}
	}

	var valA, valB any
	if dataA != nil {
		valA, err = parsePlist(dataA)
		if err != nil {
			return nil, fmt.Errorf("parsing plist A: %w", err)
		}
	}
	if dataB != nil {
		valB, err = parsePlist(dataB)
		if err != nil {
			return nil, fmt.Errorf("parsing plist B: %w", err)
		}
	}

	changes := diffPlistValues("", valA, valB)
	return &PlistDiff{Changes: changes}, nil
}

// parsePlist parses plist data (XML or binary) into a Go value.
// Binary plists are converted to XML via plutil (macOS only).
func parsePlist(data []byte) (any, error) {
	if isBinaryPlist(data) {
		xmlData, err := convertBinaryPlist(data)
		if err != nil {
			return nil, err
		}
		data = xmlData
	}
	return parsePlistXML(data)
}

func isBinaryPlist(data []byte) bool {
	return bytes.HasPrefix(data, []byte("bplist"))
}

// convertBinaryPlist converts binary plist data to XML using plutil.
// plutil is available on macOS; on other platforms this returns an error.
func convertBinaryPlist(data []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "plist-*.plist")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("plutil", "-convert", "xml1", "-o", "-", tmp.Name())
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("plutil not found (binary plist conversion is only supported on macOS)")
		}
		return nil, fmt.Errorf("plutil: %s", msg)
	}
	return stdout.Bytes(), nil
}

// --- XML Plist Parser ---

func parsePlistXML(data []byte) (any, error) {
	p := &plistParser{dec: xml.NewDecoder(bytes.NewReader(data))}
	return p.parseRoot()
}

type plistParser struct {
	dec *xml.Decoder
}

// nextStart skips whitespace, comments, and directives, returning the next
// StartElement or EndElement token.
func (p *plistParser) nextSignificant() (xml.Token, error) {
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		switch tok.(type) {
		case xml.StartElement, xml.EndElement:
			return tok, nil
		}
	}
}

func (p *plistParser) parseRoot() (any, error) {
	for {
		tok, err := p.nextSignificant()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "plist" {
			val, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			return val, nil
		}
	}
}

func (p *plistParser) parseValue() (any, error) {
	tok, err := p.nextSignificant()
	if err != nil {
		return nil, err
	}
	se, ok := tok.(xml.StartElement)
	if !ok {
		return nil, nil // end element reached (empty container)
	}
	return p.parseElement(se)
}

func (p *plistParser) parseElement(se xml.StartElement) (any, error) {
	switch se.Name.Local {
	case "dict":
		return p.parseDict()
	case "array":
		return p.parseArray()
	case "string", "integer", "real", "date", "data":
		return p.parseTextContent()
	case "true":
		p.dec.Skip()
		return true, nil
	case "false":
		p.dec.Skip()
		return false, nil
	default:
		p.dec.Skip()
		return nil, nil
	}
}

func (p *plistParser) parseDict() (map[string]any, error) {
	m := make(map[string]any)
	for {
		tok, err := p.nextSignificant()
		if err != nil {
			return nil, err
		}
		if _, ok := tok.(xml.EndElement); ok {
			return m, nil
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "key" {
			return nil, fmt.Errorf("plist: expected <key>, got %v", tok)
		}
		key, err := p.parseTextContent()
		if err != nil {
			return nil, err
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		m[key.(string)] = val
	}
}

func (p *plistParser) parseArray() ([]any, error) {
	var arr []any
	for {
		tok, err := p.nextSignificant()
		if err != nil {
			return nil, err
		}
		if _, ok := tok.(xml.EndElement); ok {
			return arr, nil
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		val, err := p.parseElement(se)
		if err != nil {
			return nil, err
		}
		arr = append(arr, val)
	}
}

func (p *plistParser) parseTextContent() (any, error) {
	var content strings.Builder
	for {
		tok, err := p.dec.Token()
		if err != nil {
			return nil, err
		}
		if cd, ok := tok.(xml.CharData); ok {
			content.Write(cd)
		}
		if _, ok := tok.(xml.EndElement); ok {
			return content.String(), nil
		}
	}
}

// --- Plist Diffing ---

func diffPlistValues(prefix string, a, b any) []PlistChange {
	if a == nil && b == nil {
		return nil
	}

	// If either side is a dict, compare as dicts.
	dictA, aDict := a.(map[string]any)
	dictB, bDict := b.(map[string]any)
	if aDict || bDict {
		if !aDict {
			dictA = map[string]any{}
		}
		if !bDict {
			dictB = map[string]any{}
		}
		return diffPlistDicts(prefix, dictA, dictB)
	}

	// If either side is an array, compare as arrays.
	arrA, aArr := a.([]any)
	arrB, bArr := b.([]any)
	if aArr || bArr {
		if !aArr {
			arrA = []any{}
		}
		if !bArr {
			arrB = []any{}
		}
		return diffPlistArrays(prefix, arrA, arrB)
	}

	// Leaf values.
	strA := formatPlistValue(a)
	strB := formatPlistValue(b)
	if a == nil {
		return []PlistChange{{KeyPath: prefix, Status: Added, ValueB: strB}}
	}
	if b == nil {
		return []PlistChange{{KeyPath: prefix, Status: Removed, ValueA: strA}}
	}
	if strA != strB {
		return []PlistChange{{KeyPath: prefix, Status: Modified, ValueA: strA, ValueB: strB}}
	}
	return nil
}

func diffPlistDicts(prefix string, a, b map[string]any) []PlistChange {
	allKeys := make(map[string]struct{})
	for k := range a {
		allKeys[k] = struct{}{}
	}
	for k := range b {
		allKeys[k] = struct{}{}
	}

	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var changes []PlistChange
	for _, k := range sorted {
		keyPath := k
		if prefix != "" {
			keyPath = prefix + "." + k
		}

		valA, inA := a[k]
		valB, inB := b[k]

		switch {
		case inA && !inB:
			changes = append(changes, PlistChange{
				KeyPath: keyPath, Status: Removed, ValueA: formatPlistValue(valA),
			})
		case !inA && inB:
			changes = append(changes, PlistChange{
				KeyPath: keyPath, Status: Added, ValueB: formatPlistValue(valB),
			})
		default:
			changes = append(changes, diffPlistValues(keyPath, valA, valB)...)
		}
	}
	return changes
}

func diffPlistArrays(prefix string, a, b []any) []PlistChange {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	var changes []PlistChange
	for i := 0; i < maxLen; i++ {
		keyPath := fmt.Sprintf("%s[%d]", prefix, i)
		switch {
		case i >= len(a):
			changes = append(changes, PlistChange{
				KeyPath: keyPath, Status: Added, ValueB: formatPlistValue(b[i]),
			})
		case i >= len(b):
			changes = append(changes, PlistChange{
				KeyPath: keyPath, Status: Removed, ValueA: formatPlistValue(a[i]),
			})
		default:
			changes = append(changes, diffPlistValues(keyPath, a[i], b[i])...)
		}
	}
	return changes
}

func formatPlistValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case map[string]any:
		return fmt.Sprintf("<dict: %d keys>", len(val))
	case []any:
		return fmt.Sprintf("<array: %d items>", len(val))
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}
