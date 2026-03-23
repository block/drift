package compare

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// gitSourcePrefix is the scheme prefix used to identify git sources in
// readContent, contentHash, and prepareBinaryPath. It is an internal
// implementation detail and should not appear in the public Result type.
const gitSourcePrefix = "git::"

// gitFileChange represents a single file change from git diff --name-status.
type gitFileChange struct {
	status DiffStatus
	path   string
}

// --- Git CLI helpers ---

// runGit executes a git command and returns its stdout as a string.
func runGit(args ...string) (string, error) {
	stdout, err := runGitRaw(args...)
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

// runGitBytes executes a git command and returns raw stdout bytes.
// Used for git show where output may be binary.
func runGitBytes(args ...string) ([]byte, error) {
	return runGitRaw(args...)
}

// runGitRaw executes a git command and returns raw stdout bytes.
func runGitRaw(args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("git not found (install git: https://git-scm.com)")
		}
		if errMsg := strings.TrimSpace(stderr.String()); errMsg != "" {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), errMsg)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// --- Repo utilities ---

// IsGitRepo returns true if the current working directory is inside a git repository.
func IsGitRepo() bool {
	_, err := runGit("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// ResolveGitRef validates and resolves a git ref to its full SHA.
func ResolveGitRef(ref string) (string, error) {
	out, err := runGit("rev-parse", "--verify", ref)
	if err != nil {
		return "", fmt.Errorf("not a valid git ref: %s", ref)
	}
	return strings.TrimSpace(out), nil
}

// gitTopLevel returns the root directory of the current git repository.
//
// The result is cached for the lifetime of the process via sync.Once.
// This is safe for the CLI (one invocation per process) but would need
// refactoring for library or multi-repo usage.
var (
	cachedTopLevel string
	topLevelOnce   sync.Once
	topLevelErr    error
)

func gitTopLevel() (string, error) {
	topLevelOnce.Do(func() {
		out, err := runGit("rev-parse", "--show-toplevel")
		if err != nil {
			topLevelErr = err
			return
		}
		cachedTopLevel = strings.TrimSpace(out)
	})
	return cachedTopLevel, topLevelErr
}

// resetGitCaches clears all cached git state. Used by tests to isolate
// each test case that creates its own temporary git repository.
func resetGitCaches() {
	cachedTopLevel = ""
	topLevelOnce = sync.Once{}
	topLevelErr = nil
	cachedRemoteURL = ""
	remoteOnce = sync.Once{}
}

// --- Source dispatch ---

// isGitSource returns true if the source string uses the git:: prefix scheme.
func isGitSource(source string) bool {
	return strings.HasPrefix(source, gitSourcePrefix)
}

// gitRef extracts the git ref from a git:: prefixed source string.
func gitRef(source string) string {
	return strings.TrimPrefix(source, gitSourcePrefix)
}

// gitSourcePaths returns the git:: prefixed source paths for use by
// readContent, contentHash, and prepareBinaryPath.
func gitSourcePaths(result *Result) (string, string) {
	return gitSourcePrefix + result.PathA, gitSourcePrefix + result.PathB
}

// readGitContent reads file content from a git ref or the working tree.
func readGitContent(ref, relPath string) ([]byte, error) {
	if ref == "worktree" {
		top, err := gitTopLevel()
		if err != nil {
			return nil, err
		}
		return os.ReadFile(filepath.Join(top, relPath))
	}
	return runGitBytes("show", ref+":"+relPath)
}

// --- Tree builders ---

// compareGit builds a diff tree between two git refs.
func compareGit(refA, refB string) (*Node, error) {
	diffOut, err := runGit("diff", "--name-status", "--no-renames", refA, refB)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	changes := parseGitDiffNameStatus(diffOut)

	if len(changes) == 0 {
		return &Node{
			Name:  shortRef(refA) + " ↔ " + shortRef(refB),
			IsDir: true,
			Kind:  KindDirectory,
		}, nil
	}

	sizesA, err := gitFileSizes(refA)
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %w", refA, err)
	}
	sizesB, err := gitFileSizes(refB)
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %w", refB, err)
	}

	nodes := buildNodesFromChanges(changes, sizesA, sizesB)
	return buildTree(nodes, shortRef(refA)+" ↔ "+shortRef(refB)), nil
}

// CompareGitWorkTree builds a diff tree of uncommitted changes against HEAD.
func CompareGitWorkTree() (*Result, error) {
	top, err := gitTopLevel()
	if err != nil {
		return nil, err
	}

	changes, err := collectWorkTreeChanges()
	if err != nil {
		return nil, err
	}

	meta := BuildGitMeta("HEAD", "worktree")

	if len(changes) == 0 {
		root := &Node{
			Name:  "HEAD ↔ working tree",
			IsDir: true,
			Kind:  KindDirectory,
		}
		return &Result{
			PathA:   "HEAD",
			PathB:   "worktree",
			Mode:    ModeGit,
			Root:    root,
			Summary: ComputeSummary(root),
			Git:     meta,
		}, nil
	}

	sizesA, err := gitFileSizes("HEAD")
	if err != nil {
		return nil, fmt.Errorf("ls-tree HEAD: %w", err)
	}

	sizesB := workTreeSizes(top, changes)

	nodes := buildNodesFromChanges(changes, sizesA, sizesB)
	root := buildTree(nodes, "HEAD ↔ working tree")

	return &Result{
		PathA:   "HEAD",
		PathB:   "worktree",
		Mode:    ModeGit,
		Root:    root,
		Summary: ComputeSummary(root),
		Git:     meta,
	}, nil
}

// collectWorkTreeChanges gathers tracked, staged, and untracked file changes.
func collectWorkTreeChanges() ([]gitFileChange, error) {
	// Unstaged tracked changes.
	diffOut, err := runGit("diff", "--name-status", "--no-renames", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	changes := parseGitDiffNameStatus(diffOut)

	// Staged changes not yet in HEAD.
	stagedOut, err := runGit("diff", "--name-status", "--no-renames", "--cached", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git diff --cached: %w", err)
	}

	seen := make(map[string]bool, len(changes))
	for _, c := range changes {
		seen[c.path] = true
	}
	for _, c := range parseGitDiffNameStatus(stagedOut) {
		if !seen[c.path] {
			changes = append(changes, c)
			seen[c.path] = true
		}
	}

	// Untracked files.
	untrackedOut, err := runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(untrackedOut), "\n") {
		if path := strings.TrimSpace(line); path != "" && !seen[path] {
			changes = append(changes, gitFileChange{status: Added, path: path})
		}
	}

	return changes, nil
}

// workTreeSizes collects file sizes from the working tree via os.Stat.
func workTreeSizes(top string, changes []gitFileChange) map[string]int64 {
	sizes := make(map[string]int64, len(changes))
	for _, c := range changes {
		if c.status == Removed {
			continue
		}
		if info, err := os.Stat(filepath.Join(top, c.path)); err == nil {
			sizes[c.path] = info.Size()
		}
	}
	return sizes
}

// --- Parsers ---

// parseGitDiffNameStatus parses the output of git diff --name-status.
func parseGitDiffNameStatus(output string) []gitFileChange {
	var changes []gitFileChange
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Standard format: "M\tpath/to/file"
		statusCode, path, ok := strings.Cut(line, "\t")
		if !ok {
			// Fallback: space-separated.
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			statusCode, path = parts[0], parts[len(parts)-1]
		}

		var status DiffStatus
		switch {
		case statusCode == "A":
			status = Added
		case statusCode == "D":
			status = Removed
		default:
			// M, T, and any other status treated as modified.
			status = Modified
		}

		changes = append(changes, gitFileChange{status: status, path: path})
	}
	return changes
}

// parseGitLsTree parses the output of git ls-tree -r -l and returns a path-to-size map.
func parseGitLsTree(output string) map[string]int64 {
	sizes := make(map[string]int64)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <mode> <type> <hash> <size>\t<path>
		meta, path, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) < 4 || fields[3] == "-" {
			continue
		}
		if size, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
			sizes[path] = size
		}
	}
	return sizes
}

// gitFileSizes returns a map of file path to size for all files in a git ref.
func gitFileSizes(ref string) (map[string]int64, error) {
	out, err := runGit("ls-tree", "-r", "-l", ref)
	if err != nil {
		return nil, err
	}
	return parseGitLsTree(out), nil
}

// buildNodesFromChanges converts parsed git changes into a Node slice for buildTree.
func buildNodesFromChanges(changes []gitFileChange, sizesA, sizesB map[string]int64) []*Node {
	nodes := make([]*Node, 0, len(changes))
	for _, c := range changes {
		nodes = append(nodes, &Node{
			Name:   filepath.Base(c.path),
			Path:   c.path,
			Status: c.status,
			Kind:   classifyPath(c.path, false),
			SizeA:  sizesA[c.path],
			SizeB:  sizesB[c.path],
		})
	}
	return nodes
}

// --- Git metadata ---

// gitCommitInfo retrieves commit metadata for a given ref.
// Returns nil for special refs like "worktree" that aren't real commits.
func gitCommitInfo(ref string) *GitCommitInfo {
	if ref == "worktree" {
		return nil
	}
	// Use %x00 (null byte) as separator for reliable parsing.
	out, err := runGit("log", "-1", "--format=%H%x00%s%x00%b%x00%an%x00%ai", ref)
	if err != nil {
		return nil
	}
	parts := strings.SplitN(strings.TrimSpace(out), "\x00", 5)
	if len(parts) < 5 {
		return nil
	}
	return &GitCommitInfo{
		SHA:     parts[0],
		Subject: parts[1],
		Body:    strings.TrimSpace(parts[2]),
		Author:  parts[3],
		Date:    parts[4],
		Ref:     ref,
		Remote:  gitRemoteURL(),
	}
}

// gitRemoteURL returns the remote origin URL for constructing web links.
// Cached for the process lifetime (see gitTopLevel for rationale).
var (
	cachedRemoteURL string
	remoteOnce      sync.Once
)

func gitRemoteURL() string {
	remoteOnce.Do(func() {
		out, err := runGit("remote", "get-url", "origin")
		if err != nil {
			return
		}
		cachedRemoteURL = normalizeRemoteURL(strings.TrimSpace(out))
	})
	return cachedRemoteURL
}

// normalizeRemoteURL converts a git remote URL to an HTTPS base URL.
// Supports SSH protocol (ssh://user@host/path), SCP-style (git@host:path),
// and HTTPS URLs. Unsupported formats (file://, local paths) are returned as-is.
func normalizeRemoteURL(raw string) string {
	// SSH protocol: ssh://user@github.com/org/repo.git -> https://github.com/org/repo
	if after, ok := strings.CutPrefix(raw, "ssh://"); ok {
		if _, host, found := strings.Cut(after, "@"); found {
			raw = "https://" + host
		} else {
			raw = "https://" + after
		}
	}
	// SCP-style: git@github.com:org/repo.git -> https://github.com/org/repo
	if after, ok := strings.CutPrefix(raw, "git@"); ok {
		raw = "https://" + strings.Replace(after, ":", "/", 1)
	}
	return strings.TrimSuffix(raw, ".git")
}

// CommitURL returns the web URL for viewing a commit, or empty if unavailable.
func (c *GitCommitInfo) CommitURL() string {
	if c == nil || c.Remote == "" || c.SHA == "" {
		return ""
	}
	return c.Remote + "/commit/" + c.SHA
}

// PRNumber extracts a pull request number from the commit subject
// if it follows the squash-merge "(#1234)" convention used by GitHub.
func (c *GitCommitInfo) PRNumber() string {
	if c == nil {
		return ""
	}
	idx := strings.LastIndex(c.Subject, "(#")
	if idx < 0 {
		return ""
	}
	rest := c.Subject[idx+2:]
	num, _, ok := strings.Cut(rest, ")")
	if !ok {
		return ""
	}
	for _, ch := range num {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return num
}

// PRURL returns the web URL for viewing the pull request, or empty if unavailable.
func (c *GitCommitInfo) PRURL() string {
	num := c.PRNumber()
	if num == "" || c.Remote == "" {
		return ""
	}
	return c.Remote + "/pull/" + num
}

// BuildGitMeta constructs GitMeta for two refs.
func BuildGitMeta(refA, refB string) *GitMeta {
	commitA := gitCommitInfo(refA)
	commitB := gitCommitInfo(refB)
	if commitA == nil && commitB == nil {
		return nil
	}
	return &GitMeta{
		CommitA: commitA,
		CommitB: commitB,
	}
}

// --- Display helpers ---

// shortRef returns a short display name for a git ref.
// Full SHAs are truncated to 8 chars; branch names are kept as-is.
func shortRef(ref string) string {
	if len(ref) >= 40 && isHex(ref) {
		return ref[:8]
	}
	return ref
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
