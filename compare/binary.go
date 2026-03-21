package compare

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// BinaryDiff contains symbol and section differences between two Mach-O binaries.
type BinaryDiff struct {
	Symbols  []SymbolChange  `json:"symbols"`
	Sections []SectionChange `json:"sections"`
}

// SymbolChange represents a symbol that was added or removed.
type SymbolChange struct {
	Name   string     `json:"name"`
	Status DiffStatus `json:"status"`
}

// SectionChange represents a size change in a Mach-O section.
type SectionChange struct {
	Segment string `json:"segment"`
	Section string `json:"section"`
	SizeA   int64  `json:"size_a"`
	SizeB   int64  `json:"size_b"`
}

// compareBinary extracts symbols and sections from two Mach-O binaries and diffs them.
func compareBinary(sourceA, sourceB, relPath string, status DiffStatus) (*BinaryDiff, error) {
	var symsA, symsB map[string]bool
	var secsA, secsB []sectionInfo

	if status != Added {
		var err error
		symsA, secsA, err = extractFromBinary(sourceA, relPath)
		if err != nil {
			return nil, fmt.Errorf("binary A: %w", err)
		}
	}
	if status != Removed {
		var err error
		symsB, secsB, err = extractFromBinary(sourceB, relPath)
		if err != nil {
			return nil, fmt.Errorf("binary B: %w", err)
		}
	}

	return &BinaryDiff{
		Symbols:  diffSymbols(symsA, symsB),
		Sections: diffSections(secsA, secsB),
	}, nil
}

// extractFromBinary prepares a path and extracts symbols + sections from a Mach-O binary.
func extractFromBinary(source, relPath string) (map[string]bool, []sectionInfo, error) {
	path, cleanup, err := prepareBinaryPath(source, relPath)
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()

	syms, err := extractSymbols(path)
	if err != nil {
		return nil, nil, err
	}
	secs, err := extractSections(path)
	if err != nil {
		return nil, nil, err
	}
	return syms, secs, nil
}

// prepareBinaryPath returns a filesystem path to the binary. For archives,
// it extracts the file to a temp location and returns a cleanup function.
func prepareBinaryPath(source, relPath string) (string, func(), error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", nil, err
	}
	if info.IsDir() {
		if ap, inner := splitArchivePath(source, relPath); ap != "" {
			return extractToTemp(ap, inner)
		}
		return filepath.Join(source, relPath), func() {}, nil
	}
	if !isArchive(source) {
		// Standalone file: use it directly.
		return source, func() {}, nil
	}
	// Archive: extract to temp file.
	data, err := readFileFromArchive(source, relPath)
	if err != nil {
		return "", nil, err
	}
	return writeToTempFile(data)
}

// extractToTemp reads a file from an archive and writes it to a temp file.
func extractToTemp(archivePath, innerPath string) (string, func(), error) {
	data, err := readFileFromArchive(archivePath, innerPath)
	if err != nil {
		return "", nil, err
	}
	return writeToTempFile(data)
}

// writeToTempFile writes data to a temporary file and returns its path and cleanup function.
func writeToTempFile(data []byte) (string, func(), error) {
	tmp, err := os.CreateTemp("", "macho-*")
	if err != nil {
		return "", nil, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", nil, err
	}
	tmp.Close()
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil
}

// --- Symbol extraction via nm ---

func extractSymbols(path string) (map[string]bool, error) {
	output, err := runMachoTool("nm", "-gUj", path)
	if err != nil {
		return nil, err
	}
	return parseNmOutput(output), nil
}

func parseNmOutput(output string) map[string]bool {
	syms := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		syms[name] = true
	}
	return syms
}

func diffSymbols(symsA, symsB map[string]bool) []SymbolChange {
	if symsA == nil {
		symsA = map[string]bool{}
	}
	if symsB == nil {
		symsB = map[string]bool{}
	}

	var changes []SymbolChange
	for name := range symsA {
		if !symsB[name] {
			changes = append(changes, SymbolChange{Name: name, Status: Removed})
		}
	}
	for name := range symsB {
		if !symsA[name] {
			changes = append(changes, SymbolChange{Name: name, Status: Added})
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Status != changes[j].Status {
			return changes[i].Status < changes[j].Status
		}
		return changes[i].Name < changes[j].Name
	})
	return changes
}

// --- Section extraction via size ---

type sectionInfo struct {
	segment string
	section string
	size    int64
}

var (
	segmentRe = regexp.MustCompile(`^Segment (\S+): (\d+)`)
	sectionRe = regexp.MustCompile(`^\s+Section (\S+): (\d+)`)
)

func extractSections(path string) ([]sectionInfo, error) {
	output, err := runMachoTool("size", "-m", path)
	if err != nil {
		return nil, err
	}
	return parseSizeOutput(output), nil
}

func parseSizeOutput(output string) []sectionInfo {
	var sections []sectionInfo
	var currentSegment string

	for _, line := range strings.Split(output, "\n") {
		if m := segmentRe.FindStringSubmatch(line); m != nil {
			currentSegment = m[1]
			continue
		}
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			size, err := strconv.ParseInt(m[2], 10, 64)
			if err != nil {
				continue
			}
			sections = append(sections, sectionInfo{
				segment: currentSegment,
				section: m[1],
				size:    size,
			})
		}
	}
	return sections
}

func diffSections(secsA, secsB []sectionInfo) []SectionChange {
	indexA := indexSections(secsA)
	indexB := indexSections(secsB)

	allKeys := make(map[string]struct{})
	for k := range indexA {
		allKeys[k] = struct{}{}
	}
	for k := range indexB {
		allKeys[k] = struct{}{}
	}

	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var changes []SectionChange
	for _, key := range sorted {
		a := indexA[key]
		b := indexB[key]
		if a.size == b.size {
			continue
		}
		segment := a.segment
		section := a.section
		if segment == "" {
			segment = b.segment
			section = b.section
		}
		changes = append(changes, SectionChange{
			Segment: segment,
			Section: section,
			SizeA:   a.size,
			SizeB:   b.size,
		})
	}
	return changes
}

func indexSections(secs []sectionInfo) map[string]sectionInfo {
	m := make(map[string]sectionInfo, len(secs))
	for _, s := range secs {
		key := s.segment + "." + s.section
		m[key] = s
	}
	return m
}

// --- Tool execution ---

func runMachoTool(bin string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("%s not found (%s)", bin, installHint())
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("%s: %s", bin, errMsg)
		}
		return "", fmt.Errorf("%s: %w", bin, err)
	}
	return stdout.String(), nil
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "install Xcode CLI Tools: xcode-select --install"
	case "linux":
		return "install binutils via your package manager"
	default:
		return "not available on this platform"
	}
}
