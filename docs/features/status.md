# status

## Purpose

Check whether project indexes are current compared to the current filesystem state.

## Usage

```bash
idx status
idx status --profile
```

## Arguments

- None.

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--profile` | bool | `false` | Show a detailed per-directory report and summary table |
| `--quiet`, `-q` | bool | `false` | Suppress informational output |

## Behavior and Side Effects

- Resolves current directory and project Git root.
- Loads ignore rules from the project root.
- Discovers indexed directories under the project.
- Discovers eligible directories (non-ignored) and keeps only directories that currently contain indexable files.
- Fails if eligible directories with files exist but are not indexed.
- For each indexed directory, checks whether reindexing is needed using current file state and checksum/index snapshot logic.
- Marks a directory as stale when it requires reindexing.
- With `--profile`, prints one panel per indexed directory plus a summary panel before final status validation.
- Read-only command; no writes are performed.

## Output

- Success:
  - `✅ Indices are up to date.`
- With `--profile`:
  - Per-directory sections with:
    - `📂` directory label
    - `✅ updated` or `❌ stale`
    - File table with columns: `file`, `updated`, `modified_at`
    - Optional note when file set changed since last indexing
  - Summary section:
    - `📊 Summary`
    - `directories checked`
    - `directories updated`
    - `files checked`
    - `files updated`
    - `latest file modification`

## Errors

- No index found under project root:
  - `no index found under project root "<root>": run idx init first`
- Unindexed eligible directories found:
  - `unindexed directories found: [...] — run idx sync to update`
- Stale index detected:
  - `stale index at "<directory>": run idx sync to update`
- Current directory/Git root resolution failures.
- Ignore matcher loading failures.
- Directory listing or file-state read failures while checking status.

## Examples

```bash
idx status
idx status --profile
```
