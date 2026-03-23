package compare

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// --- Unit tests (no git repo needed) ---

func TestParseGitDiffNameStatus(t *testing.T) {
	input := "A\tpath/to/new.txt\nD\tpath/to/removed.txt\nM\tpath/to/changed.txt\n"
	changes := parseGitDiffNameStatus(input)
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(changes))
	}

	tests := []struct {
		path   string
		status DiffStatus
	}{
		{"path/to/new.txt", Added},
		{"path/to/removed.txt", Removed},
		{"path/to/changed.txt", Modified},
	}
	for i, tt := range tests {
		if changes[i].path != tt.path {
			t.Errorf("change[%d].path = %q, want %q", i, changes[i].path, tt.path)
		}
		if changes[i].status != tt.status {
			t.Errorf("change[%d].status = %v, want %v", i, changes[i].status, tt.status)
		}
	}
}

func TestParseGitDiffNameStatus_Empty(t *testing.T) {
	changes := parseGitDiffNameStatus("")
	if len(changes) != 0 {
		t.Errorf("got %d changes, want 0", len(changes))
	}
}

func TestParseGitDiffNameStatus_TypeChange(t *testing.T) {
	changes := parseGitDiffNameStatus("T\tfile.txt\n")
	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1", len(changes))
	}
	if changes[0].status != Modified {
		t.Errorf("status = %v, want Modified (type change)", changes[0].status)
	}
}

func TestParseGitDiffNameStatus_MalformedLines(t *testing.T) {
	input := "garbage\n\nM\tvalid.txt\n  \n"
	changes := parseGitDiffNameStatus(input)
	if len(changes) != 1 {
		t.Fatalf("got %d changes, want 1 (malformed lines skipped)", len(changes))
	}
	if changes[0].path != "valid.txt" {
		t.Errorf("path = %q, want valid.txt", changes[0].path)
	}
}

func TestParseGitLsTree(t *testing.T) {
	input := "100644 blob abc123 1234\tpath/to/file.txt\n" +
		"100644 blob def456 5678\tanother/file.go\n" +
		"040000 tree 000000 -\tsome/directory\n"
	sizes := parseGitLsTree(input)
	if len(sizes) != 2 {
		t.Fatalf("got %d entries, want 2 (directories excluded)", len(sizes))
	}
	if sizes["path/to/file.txt"] != 1234 {
		t.Errorf("file.txt size = %d, want 1234", sizes["path/to/file.txt"])
	}
	if sizes["another/file.go"] != 5678 {
		t.Errorf("file.go size = %d, want 5678", sizes["another/file.go"])
	}
}

func TestParseGitLsTree_Empty(t *testing.T) {
	sizes := parseGitLsTree("")
	if len(sizes) != 0 {
		t.Errorf("got %d entries, want 0", len(sizes))
	}
}

func TestBuildNodesFromChanges(t *testing.T) {
	changes := []gitFileChange{
		{status: Added, path: "src/new.go"},
		{status: Removed, path: "src/old.go"},
		{status: Modified, path: "README.md"},
	}
	sizesA := map[string]int64{"src/old.go": 100, "README.md": 200}
	sizesB := map[string]int64{"src/new.go": 150, "README.md": 250}

	nodes := buildNodesFromChanges(changes, sizesA, sizesB)
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}

	for _, n := range nodes {
		switch n.Path {
		case "src/new.go":
			if n.Status != Added || n.SizeB != 150 {
				t.Errorf("new.go: status=%v SizeB=%d, want Added/150", n.Status, n.SizeB)
			}
		case "src/old.go":
			if n.Status != Removed || n.SizeA != 100 {
				t.Errorf("old.go: status=%v SizeA=%d, want Removed/100", n.Status, n.SizeA)
			}
		case "README.md":
			if n.Status != Modified || n.SizeA != 200 || n.SizeB != 250 {
				t.Errorf("README.md: status=%v sizes=%d/%d, want Modified/200/250", n.Status, n.SizeA, n.SizeB)
			}
		}
	}
}

func TestShortRef(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"main", "main"},
		{"HEAD", "HEAD"},
		{"HEAD~3", "HEAD~3"},
		{"abc123def456abc123def456abc123def456abc123", "abc123de"},
	}
	for _, tt := range tests {
		if got := shortRef(tt.input); got != tt.want {
			t.Errorf("shortRef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		// SCP-style SSH
		{"git@github.com:org/repo.git", "https://github.com/org/repo"},
		// SSH protocol with user
		{"ssh://org-12345@github.com/org/repo.git", "https://github.com/org/repo"},
		// SSH protocol without user
		{"ssh://github.com/org/repo.git", "https://github.com/org/repo"},
		// HTTPS already normalized
		{"https://github.com/org/repo.git", "https://github.com/org/repo"},
		{"https://github.com/org/repo", "https://github.com/org/repo"},
		// No .git suffix
		{"git@github.com:org/repo", "https://github.com/org/repo"},
	}
	for _, tt := range tests {
		if got := normalizeRemoteURL(tt.input); got != tt.want {
			t.Errorf("normalizeRemoteURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPRNumber(t *testing.T) {
	tests := []struct {
		subject string
		want    string
	}{
		{"feat: add feature (#1234)", "1234"},
		{"HOOD-219: Add tab badge (#79643)", "79643"},
		{"no pr number here", ""},
		{"has parens but not pr (foo)", ""},
		{"multiple (#111) refs (#222)", "222"},
		{"(#abc) not digits", ""},
		{"unclosed (#123", ""},
	}
	for _, tt := range tests {
		c := &GitCommitInfo{Subject: tt.subject}
		if got := c.PRNumber(); got != tt.want {
			t.Errorf("PRNumber(%q) = %q, want %q", tt.subject, got, tt.want)
		}
	}
}

func TestPRNumber_NilReceiver(t *testing.T) {
	var c *GitCommitInfo
	if got := c.PRNumber(); got != "" {
		t.Errorf("PRNumber(nil) = %q, want empty", got)
	}
}

func TestCommitURL(t *testing.T) {
	c := &GitCommitInfo{SHA: "abc123", Remote: "https://github.com/org/repo"}
	if got := c.CommitURL(); got != "https://github.com/org/repo/commit/abc123" {
		t.Errorf("CommitURL() = %q", got)
	}

	// No remote.
	c.Remote = ""
	if got := c.CommitURL(); got != "" {
		t.Errorf("CommitURL(no remote) = %q, want empty", got)
	}
}

func TestPRURL(t *testing.T) {
	c := &GitCommitInfo{
		Subject: "feat: thing (#42)",
		Remote:  "https://github.com/org/repo",
	}
	if got := c.PRURL(); got != "https://github.com/org/repo/pull/42" {
		t.Errorf("PRURL() = %q", got)
	}
}

func TestIsGitSource(t *testing.T) {
	if !isGitSource("git::HEAD") {
		t.Error("git::HEAD should be a git source")
	}
	if isGitSource("/some/path") {
		t.Error("/some/path should not be a git source")
	}
}

func TestGitRef(t *testing.T) {
	if got := gitRef("git::HEAD"); got != "HEAD" {
		t.Errorf("gitRef(git::HEAD) = %q, want HEAD", got)
	}
}

// --- Integration tests (require git) ---

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

// initGitRepo creates a temp git repo with an initial commit and returns the path.
// It also resets global caches and changes the working directory to the repo.
// The caller should defer the returned cleanup function.
func initGitRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	origDir, _ := os.Getwd()

	resetGitCaches()
	os.Chdir(dir)

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	git("init")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "test")

	writeFile(t, filepath.Join(dir, "file.txt"), "original content\n")
	writeFile(t, filepath.Join(dir, "unchanged.txt"), "stays the same\n")
	writeFile(t, filepath.Join(dir, "removed.txt"), "will be removed\n")
	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	writeFile(t, filepath.Join(dir, "sub", "nested.txt"), "nested original\n")
	git("add", "-A")
	git("commit", "-m", "initial commit")

	cleanup := func() {
		os.Chdir(origDir)
		resetGitCaches()
	}

	return dir, cleanup
}

// gitInRepo runs a git command in the given directory.
func gitInRepo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestCompareGit_TwoCommits(t *testing.T) {
	requireGit(t)
	dir, cleanup := initGitRepo(t)
	defer cleanup()

	writeFile(t, filepath.Join(dir, "file.txt"), "modified content\n")
	os.Remove(filepath.Join(dir, "removed.txt"))
	writeFile(t, filepath.Join(dir, "added.txt"), "new file\n")
	writeFile(t, filepath.Join(dir, "sub", "nested.txt"), "nested modified\n")
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-m", "second commit")

	result, err := Compare("HEAD~1", "HEAD", ModeGit)
	if err != nil {
		t.Fatalf("Compare git: %v", err)
	}

	if result.Mode != ModeGit {
		t.Errorf("mode = %q, want git", result.Mode)
	}

	// PathA/PathB should be raw refs, not git:: prefixed.
	if result.PathA != "HEAD~1" {
		t.Errorf("PathA = %q, want HEAD~1", result.PathA)
	}
	if result.PathB != "HEAD" {
		t.Errorf("PathB = %q, want HEAD", result.PathB)
	}

	s := result.Summary
	if s.Added != 1 {
		t.Errorf("added = %d, want 1", s.Added)
	}
	if s.Removed != 1 {
		t.Errorf("removed = %d, want 1", s.Removed)
	}
	if s.Modified != 2 {
		t.Errorf("modified = %d, want 2 (file.txt + sub/nested.txt)", s.Modified)
	}
}

func TestCompareGit_NoChanges(t *testing.T) {
	requireGit(t)
	_, cleanup := initGitRepo(t)
	defer cleanup()

	result, err := Compare("HEAD", "HEAD", ModeGit)
	if err != nil {
		t.Fatalf("Compare git: %v", err)
	}
	s := result.Summary
	if s.Added+s.Removed+s.Modified != 0 {
		t.Errorf("expected no changes, got +%d -%d ~%d", s.Added, s.Removed, s.Modified)
	}
}

func TestCompareGit_Detail(t *testing.T) {
	requireGit(t)
	dir, cleanup := initGitRepo(t)
	defer cleanup()

	writeFile(t, filepath.Join(dir, "file.txt"), "modified content\n")
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-m", "modify file")

	result, err := Compare("HEAD~1", "HEAD", ModeGit)
	if err != nil {
		t.Fatalf("Compare git: %v", err)
	}

	node := findNode(result.Root, "file.txt")
	if node == nil {
		t.Fatal("file.txt not found in tree")
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

	var hasAdded, hasRemoved bool
	for _, h := range detail.Text.Hunks {
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

func TestCompareGit_GitMeta(t *testing.T) {
	requireGit(t)
	dir, cleanup := initGitRepo(t)
	defer cleanup()

	writeFile(t, filepath.Join(dir, "file.txt"), "change\n")
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-m", "feat: test commit (#99)")

	result, err := Compare("HEAD~1", "HEAD", ModeGit)
	if err != nil {
		t.Fatalf("Compare git: %v", err)
	}

	if result.Git == nil {
		t.Fatal("expected GitMeta to be populated")
	}
	if result.Git.CommitA == nil || result.Git.CommitB == nil {
		t.Fatal("expected both commits to be populated")
	}
	if result.Git.CommitB.Author != "test" {
		t.Errorf("author = %q, want test", result.Git.CommitB.Author)
	}
	if result.Git.CommitB.PRNumber() != "99" {
		t.Errorf("PRNumber = %q, want 99", result.Git.CommitB.PRNumber())
	}
}

func TestCompareGitWorkTree(t *testing.T) {
	requireGit(t)
	dir, cleanup := initGitRepo(t)
	defer cleanup()

	writeFile(t, filepath.Join(dir, "file.txt"), "uncommitted change\n")
	writeFile(t, filepath.Join(dir, "untracked.txt"), "brand new\n")

	result, err := CompareGitWorkTree()
	if err != nil {
		t.Fatalf("CompareGitWorkTree: %v", err)
	}

	if result.Mode != ModeGit {
		t.Errorf("mode = %q, want git", result.Mode)
	}
	if result.Root.Name != "HEAD ↔ working tree" {
		t.Errorf("root name = %q, want HEAD ↔ working tree", result.Root.Name)
	}
	if result.Summary.Modified < 1 {
		t.Errorf("modified = %d, want >= 1", result.Summary.Modified)
	}
	if result.Summary.Added < 1 {
		t.Errorf("added = %d, want >= 1", result.Summary.Added)
	}
}

func TestCompareGitWorkTree_Clean(t *testing.T) {
	requireGit(t)
	_, cleanup := initGitRepo(t)
	defer cleanup()

	result, err := CompareGitWorkTree()
	if err != nil {
		t.Fatalf("CompareGitWorkTree: %v", err)
	}
	s := result.Summary
	if s.Added+s.Removed+s.Modified != 0 {
		t.Errorf("expected clean working tree, got +%d -%d ~%d", s.Added, s.Removed, s.Modified)
	}
}
