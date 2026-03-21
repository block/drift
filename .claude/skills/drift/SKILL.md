---
name: drift
description: Compare files, directories, archives, or binaries using drift. Use when asked to compare, diff, or analyze differences between two paths - builds, releases, directories, archives (.ipa, .apk, .aar, .jar, .tar.gz), binaries, plists, or text files.
argument-hint: "<path-a> <path-b> [-m mode]"
allowed-tools: Bash(drift *)
---

Compare $ARGUMENTS using `drift --json` and analyze the results.

## How to run

```sh
drift --json <path-a> <path-b>
```

Optional: force a comparison mode with `-m <mode>` where mode is one of: `tree`, `binary`, `plist`, `text`.

drift auto-detects the correct mode from the inputs. Use `-m` only when the user explicitly requests a specific mode or auto-detection picks the wrong one.

## Supported inputs

| Mode | Inputs | What it compares |
|------|--------|------------------|
| tree | Directories, archives (.ipa, .apk, .aar, .jar, .tar, .tar.gz, .tar.bz2) | File tree with per-file status |
| binary | Mach-O executables and dylibs | Sections, sizes, symbols, load commands |
| plist | Property list files (.plist) | Structured key-value diff |
| text | Everything else | Line-by-line unified diff |

## JSON output schema

Every result includes:

- `path_a`, `path_b` - the compared paths
- `mode` - detected comparison mode
- `root` - diff tree where each node has: `name`, `path`, `status` (unchanged/added/removed/modified), `kind`, `is_dir`, `size_a`, `size_b`, and optional `children`
- `summary` - aggregate counts: `added`, `removed`, `modified`, `unchanged`, `size_delta`

For single-file modes (binary, plist, text), a `detail` field is included:

- **binary**: `symbols` (added/removed symbol names), `sections` (segment/section size changes)
- **plist**: `changes` with `key_path`, `status`, and before/after values
- **text**: `hunks` with line-level diffs (`kind`: context/added/removed)

## How to analyze

1. Run `drift --json <path-a> <path-b>` and capture the output
2. Start with `summary` to give an overview of what changed
3. For tree mode, walk the `root` to identify the most significant changes (largest size deltas, added/removed files)
4. For binary mode, focus on symbol changes and section size deltas - these indicate code/data growth
5. For plist mode, highlight changed keys and their before/after values
6. For text mode, summarize the hunks by theme (e.g. "updated imports", "refactored function X")

When the output is large, use `jq` to extract specific parts rather than dumping everything:

```sh
# Just the summary
drift --json A B | jq '.summary'

# Only changed files
drift --json A B | jq '[.root | .. | select(.status? != "unchanged" and .is_dir? == false)]'

# Size delta per file, sorted
drift --json A B | jq '[.root | .. | select(.is_dir? == false and .status? == "modified")] | sort_by(.size_b - .size_a) | reverse | .[] | {path, delta: (.size_b - .size_a)}'
```

## Tips

- For large archives, the JSON can be verbose. Summarize at the directory level first, then drill into specific paths if the user asks.
- Size delta is in bytes. Convert to human-readable units (KB/MB) when presenting to the user.
- When comparing builds, highlight: new files added, files removed, largest size increases, and any unexpected changes.
