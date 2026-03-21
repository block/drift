---
name: drift
description: Compare and diff files, directories, builds, releases, archives, or binaries. Use when the user wants to compare two versions of something, diff two directories or files, analyze what changed between builds or releases, or examine differences in .ipa, .apk, .aar, .jar, .tar.gz, .tar.bz2, Mach-O binaries, plists, or text files.
argument-hint: "<path-a> <path-b> [-m mode]"
allowed-tools: Bash(drift *), Bash(jq *)
context: fork
---

Compare $ARGUMENTS using `drift --json` and analyze the results.

## How to run

```sh
drift --json <path-a> <path-b>
```

Optional: force a comparison mode with `-m <mode>` where mode is one of: `tree`, `binary`, `plist`, `text`. drift auto-detects the correct mode from the inputs - only use `-m` when the user explicitly requests a mode or auto-detection picks the wrong one.

## Supported inputs

| Mode | Inputs | What it compares |
|------|--------|------------------|
| tree | Directories, archives (.ipa, .apk, .aar, .jar, .tar, .tar.gz, .tar.bz2) | File tree with per-file status |
| binary | Mach-O executables and dylibs | Sections, sizes, symbols, load commands |
| plist | Property list files (.plist) | Structured key-value diff |
| text | Everything else | Line-by-line unified diff |

## Steps

1. Run `drift --json <path-a> <path-b>` and capture the output
2. Start with `summary` to give an overview of what changed
3. For tree mode, walk the `root` to identify the most significant changes (largest size deltas, added/removed files)
4. For binary mode, focus on symbol changes and section size deltas - these indicate code/data growth
5. For plist mode, highlight changed keys and their before/after values
6. For text mode, summarize the hunks by theme (e.g. "updated imports", "refactored function X")
7. Present size deltas in human-readable units (KB/MB)

When the output is large, use `jq` to extract specific parts rather than dumping everything. See [reference.md](reference.md) for the JSON schema and useful jq recipes.
