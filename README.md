# drift

<p align="center">
  <img src=".github/assets/drift_logo.png" alt="Drift logo" width="700">
</p>

A **fast**, **interactive** file **comparison tool**. 

Compare directories, archives, binaries, plists, and text files with a terminal UI or structured JSON output.

[![asciicast](https://asciinema.org/a/858111.svg)](https://asciinema.org/a/858111)

## Installation

```sh
go install github.com/block/drift/cmd/drift@latest
```

Or [download a pre-built binary for your platform from a release](https://github.com/block/drift/releases)

Or build from source:

```sh
gh repo clone block/drift
cd drift
go run ./cmd/drift --help
```

## Usage

```sh
# Compare two directories
drift path/a path/b

# Compare two archives (.ipa, .apk, .aar, .jar, .tar.gz, .tar.bz2)
drift app-v1.ipa app-v2.ipa

# Compare two binaries
drift bin-v1 bin-v2

# Force a specific comparison mode
drift -m binary lib-v1.dylib lib-v2.dylib

# JSON output (non-interactive, for scripting)
drift --json path/a path/b
```

### Comparison modes

drift auto-detects the comparison mode based on the inputs:

| Mode | Inputs | What it shows |
|------|--------|---------------|
| **tree** | Directories, archives | File tree with added/removed/modified indicators, per-file diffs |
| **binary** | Mach-O binaries | Sections, sizes, symbols, load commands. Requires `nm` and `size` |
| **plist** | Property lists (.plist) | Structured key-value diff. Binary plists require `plutil` |
| **text** | Everything else | Line-by-line unified diff |

Use `-m <mode>` to override auto-detection.

### Archives

drift transparently extracts and compares the contents of:
- `.ipa` (iOS app bundles)
- `.apk` (Android app bundles)
- `.aar` (Android libraries)
- `.jar` (Java archives)
- `.tar`, `.tar.gz` / `.tgz`, `.tar.bz2`

## Interactive TUI

When stdout is a terminal, drift launches an interactive Bubbletea-based TUI with a split-pane layout: file tree on the left, detail diff on the right.

### Keybindings

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Navigate tree |
| `→`/`enter`/`l` | Expand node |
| `←`/`h` | Collapse node |
| `tab` | Switch pane focus |
| `n`/`N` | Next/previous change |
| `f` | Cycle filter (all → added → removed → modified) |
| `1`-`4` | Filter: all, added, removed, modified |
| `/` | Search (fuzzy match in tree, text search in detail) |
| `s` | Swap A ↔ B |
| `c` | Copy detail to clipboard |
| `pgup`/`pgdn` | Scroll detail pane |
| `g`/`G` | Jump to top/bottom |
| `?` | Toggle full help |
| `q`/`ctrl+c` | Quit |

## JSON output

Pass `--json` to get structured JSON output suitable for scripting and CI pipelines. For tree mode, the output is the full comparison result. For single-file modes (binary, plist, text), the output includes both the summary and the detailed diff.

## Platform support

drift works on **macOS**, **Linux**, and **Windows**. Core features (directory/archive comparison, text diffing) work everywhere. Some features require external tools and degrade gracefully when they are unavailable:

| Tool | Used for | Availability |
| --- | --- | --- |
| `nm`, `size` | Mach-O binary analysis | macOS (Xcode CLI Tools), Linux (binutils) |
| `plutil` | Binary plist conversion | macOS only (XML plists work everywhere) |
| `xclip` or `xsel` | Clipboard | Linux only (macOS and Windows work natively) |

## License

[Apache License 2.0](LICENSE)
