# drift JSON reference

## Output schema

Every result includes:

- `path_a`, `path_b` - the compared paths
- `mode` - detected comparison mode (`tree`, `binary`, `plist`, `text`)
- `root` - diff tree where each node has: `name`, `path`, `status` (unchanged/added/removed/modified), `kind`, `is_dir`, `size_a`, `size_b`, and optional `children`
- `summary` - aggregate counts: `added`, `removed`, `modified`, `unchanged`, `size_delta`

For single-file modes (binary, plist, text), a `detail` field is included:

- **binary**: `symbols` (added/removed symbol names), `sections` (segment/section size changes)
- **plist**: `changes` with `key_path`, `status`, and before/after values
- **text**: `hunks` with line-level diffs (`kind`: context/added/removed)

## Useful jq recipes

```sh
# Just the summary
drift --json A B | jq '.summary'

# Only changed files (flat list)
drift --json A B | jq '[.root | .. | select(.status? != "unchanged" and .is_dir? == false)]'

# Size delta per file, sorted largest first
drift --json A B | jq '[.root | .. | select(.is_dir? == false and .status? == "modified")] | sort_by(.size_b - .size_a) | reverse | .[] | {path, delta: (.size_b - .size_a)}'

# List all added files
drift --json A B | jq '[.root | .. | select(.status? == "added") | .path]'

# Total size delta
drift --json A B | jq '.summary.size_delta'
```

## Tips

- For large archives, summarize at the directory level first, then drill into specific paths if the user asks.
- When comparing builds, highlight: new files added, files removed, largest size increases, and any unexpected changes.
