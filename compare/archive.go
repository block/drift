package compare

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// archiveFormat represents a supported archive type.
type archiveFormat int

const (
	archiveNone   archiveFormat = iota
	archiveZip                  // .zip, .ipa, .jar, .apk, .aar
	archiveTar                  // .tar
	archiveTarGz                // .tar.gz, .tgz
	archiveTarBz2               // .tar.bz2, .tbz2
)

// archiveFormatFor returns the archive format for the given path, or archiveNone.
// Checks double-extensions (.tar.gz, .tar.bz2) before single extensions.
func archiveFormatFor(path string) archiveFormat {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tar.bz2") {
		if strings.HasSuffix(lower, ".tar.gz") {
			return archiveTarGz
		}
		return archiveTarBz2
	}
	ext := filepath.Ext(lower)
	switch ext {
	case ".zip", ".ipa", ".jar", ".apk", ".aar":
		return archiveZip
	case ".tar":
		return archiveTar
	case ".tgz":
		return archiveTarGz
	case ".tbz2":
		return archiveTarBz2
	}
	return archiveNone
}

// isArchive returns true if the path has a supported archive extension.
func isArchive(path string) bool {
	return archiveFormatFor(path) != archiveNone
}

// readArchive reads an archive file and returns a flat list of file entries.
func readArchive(path string) ([]fileEntry, error) {
	switch archiveFormatFor(path) {
	case archiveZip:
		return readZipArchive(path)
	case archiveTar, archiveTarGz, archiveTarBz2:
		return readTarArchive(path)
	default:
		return nil, fmt.Errorf("%s: unsupported archive format", path)
	}
}

// readFileFromArchive extracts a single file's contents from an archive.
func readFileFromArchive(archivePath, relPath string) ([]byte, error) {
	switch archiveFormatFor(archivePath) {
	case archiveZip:
		return readFileFromZipArchive(archivePath, relPath)
	case archiveTar, archiveTarGz, archiveTarBz2:
		return readFileFromTarArchive(archivePath, relPath)
	default:
		return nil, fmt.Errorf("%s: unsupported archive format", archivePath)
	}
}

// --- ZIP ---

func readZipArchive(path string) ([]fileEntry, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	seen := make(map[string]bool)
	var entries []fileEntry

	for _, f := range r.File {
		name := strings.TrimSuffix(f.Name, "/")
		if name == "" {
			continue
		}

		ensureParentEntries(name, seen, &entries)

		if f.FileInfo().IsDir() {
			if !seen[name] {
				seen[name] = true
				entries = append(entries, fileEntry{relPath: name, isDir: true})
			}
			continue
		}

		if !seen[name] {
			seen[name] = true
			entries = append(entries, fileEntry{
				relPath: name,
				size:    int64(f.UncompressedSize64),
				isDir:   false,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})
	return entries, nil
}

func readFileFromZipArchive(archivePath, relPath string) ([]byte, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.TrimSuffix(f.Name, "/")
		if name == relPath {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("file %q not found in %s", relPath, archivePath)
}

// --- TAR ---

// openTarReader opens a tar archive with optional decompression.
func openTarReader(path string) (*tar.Reader, func() error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}

	var r io.Reader = f
	var closers []io.Closer
	closers = append(closers, f)

	switch archiveFormatFor(path) {
	case archiveTarGz:
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		closers = append(closers, gz)
		r = gz
	case archiveTarBz2:
		r = bzip2.NewReader(f)
	}

	cleanup := func() error {
		var firstErr error
		// Close in reverse order.
		for i := len(closers) - 1; i >= 0; i-- {
			if err := closers[i].Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	return tar.NewReader(r), cleanup, nil
}

func readTarArchive(path string) ([]fileEntry, error) {
	tr, cleanup, err := openTarReader(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	seen := make(map[string]bool)
	var entries []fileEntry

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := strings.TrimSuffix(hdr.Name, "/")
		if name == "" || name == "." {
			continue
		}

		ensureParentEntries(name, seen, &entries)

		if hdr.Typeflag == tar.TypeDir {
			if !seen[name] {
				seen[name] = true
				entries = append(entries, fileEntry{relPath: name, isDir: true})
			}
			continue
		}

		// Only include regular files.
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != 0 {
			continue
		}

		if !seen[name] {
			seen[name] = true
			entries = append(entries, fileEntry{
				relPath: name,
				size:    hdr.Size,
				isDir:   false,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})
	return entries, nil
}

func readFileFromTarArchive(archivePath, relPath string) ([]byte, error) {
	tr, cleanup, err := openTarReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := strings.TrimSuffix(hdr.Name, "/")
		if name == relPath {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("file %q not found in %s", relPath, archivePath)
}

// --- Shared helpers ---

// ensureParentEntries adds intermediate directory entries for all parent
// components of name that haven't been seen yet.
func ensureParentEntries(name string, seen map[string]bool, entries *[]fileEntry) {
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if !seen[dir] {
			seen[dir] = true
			*entries = append(*entries, fileEntry{relPath: dir, isDir: true})
		}
	}
}
