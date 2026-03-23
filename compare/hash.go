package compare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// contentHash computes the SHA-256 of a file within a source (directory, archive, or standalone file).
func contentHash(source, relPath string) (string, error) {
	if isGitSource(source) {
		data, err := readGitContent(gitRef(source), relPath)
		if err != nil {
			return "", err
		}
		h := sha256.Sum256(data)
		return hex.EncodeToString(h[:]), nil
	}

	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if ap, inner := splitArchivePath(source, relPath); ap != "" {
			return hashFileInArchive(ap, inner)
		}
		return hashFileOnDisk(filepath.Join(source, relPath))
	}
	if isArchive(source) {
		return hashFileInArchive(source, relPath)
	}
	// Standalone file: hash it directly.
	return hashFileOnDisk(source)
}

func hashFileOnDisk(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFileInArchive(archivePath, relPath string) (string, error) {
	data, err := readFileFromArchive(archivePath, relPath)
	if err != nil {
		return "", fmt.Errorf("hashing %s in %s: %w", relPath, archivePath, err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// readContent reads the raw bytes of a file within a source (directory, archive, or standalone file).
func readContent(source, relPath string) ([]byte, error) {
	if isGitSource(source) {
		return readGitContent(gitRef(source), relPath)
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		if ap, inner := splitArchivePath(source, relPath); ap != "" {
			return readFileFromArchive(ap, inner)
		}
		return os.ReadFile(filepath.Join(source, relPath))
	}
	if isArchive(source) {
		return readFileFromArchive(source, relPath)
	}
	// Standalone file: read it directly.
	return os.ReadFile(source)
}
