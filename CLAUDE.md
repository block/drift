# CLAUDE.md

## Build & Test

- **Build:** `go build -o drift ./cmd/drift`
- **Test:** `go test ./...` or `go test ./compare -run TestName`
- **Vet:** `go vet ./...`

## Architecture

`drift` is a two-layer tool: a comparison engine and an interactive TUI.

### Compare engine (`compare/`)

Pure-Go diffing logic. `Compare(pathA, pathB, mode)` returns a `*Result` with a tree of `Node`s representing the comparison. `Detail(result, node)` produces the detailed diff for a single node.

Supported modes: tree (directories/archives), binary (Mach-O), plist, text. Mode is auto-detected from inputs.

Key files:
- `compare.go` - entry point, mode detection, tree comparison
- `directory.go` - directory walking and tree building
- `archive.go` - archive extraction (zip, tar, tar.gz, tar.bz2)
- `binary.go` - Mach-O analysis via nm/size/otool
- `plist.go` - plist conversion via plutil
- `text.go` - line-by-line unified diff
- `types.go` - `Result`, `Node`, `DetailResult` types
- `hash.go` - content hashing for change detection

### TUI (`tui/`)

Bubbletea-based interactive terminal UI. Three-tier component model:

1. **App** (`app.go`) - root model, layout, keyboard dispatch
2. **Components** (`tree.go`, `detail.go`, `search.go`, `overlay.go`, `alert.go`) - stateful sub-models
3. **Views** (`view_*.go`, `summary.go`, `render.go`) - pure render functions, no state

- `styles.go` - all lipgloss styles
- `help.go` - keybinding definitions
- `components.go` - detail content builder

### Entry point

- `cmd/drift/main.go` - CLI struct, kong parser, `run()` method

## Code style

- Views are pure functions: `func renderX(width int, data T) string`
- Components own state and implement `Update`/`View`
- Use lipgloss for all styling - no raw ANSI
