package compare

import (
	"os"
	"path/filepath"
	"strings"
)

// walkDir walks a directory and returns a flat list of file entries.
// Archive files are expanded inline — their contents appear as children
// under the archive path, which itself is treated as a directory.
func walkDir(root string) ([]fileEntry, error) {
	var entries []fileEntry
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}

		if !d.IsDir() && isArchive(path) {
			entries = expandArchiveInline(entries, rel, path)
			return nil
		}

		entries = append(entries, fileEntry{
			relPath: rel,
			size:    info.Size(),
			isDir:   d.IsDir(),
		})
		return nil
	})
	return entries, err
}

// expandArchiveInline adds the archive as a directory entry, then appends
// all its contents prefixed with the archive's relative path. If the archive
// cannot be read, it falls back to a single opaque file entry.
func expandArchiveInline(entries []fileEntry, rel, absPath string) []fileEntry {
	archiveEntries, err := readArchive(absPath)
	if err != nil {
		// Can't read the archive — treat as opaque file.
		info, statErr := os.Stat(absPath)
		var size int64
		if statErr == nil {
			size = info.Size()
		}
		return append(entries, fileEntry{relPath: rel, size: size, isDir: false})
	}

	// Archive root appears as a directory.
	entries = append(entries, fileEntry{relPath: rel, isDir: true})
	for _, e := range archiveEntries {
		entries = append(entries, fileEntry{
			relPath: rel + "/" + e.relPath,
			size:    e.size,
			isDir:   e.isDir,
		})
	}
	return entries
}

// splitArchivePath checks whether a relative path from a directory source
// traverses through an archive file. Returns the archive's absolute path
// and the remaining inner path, or ("", "") if no archive boundary exists.
func splitArchivePath(dirSource, relPath string) (archivePath, innerPath string) {
	parts := strings.Split(relPath, "/")
	for i := 1; i <= len(parts); i++ {
		prefix := filepath.Join(parts[:i]...)
		candidate := filepath.Join(dirSource, prefix)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if !info.IsDir() && isArchive(candidate) && i < len(parts) {
			return candidate, strings.Join(parts[i:], "/")
		}
	}
	return "", ""
}
