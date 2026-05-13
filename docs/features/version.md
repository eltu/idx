# version

## Purpose

Show binary version, build date, current directory, and a styled logo.

## Usage

```bash
idx version
idx --version
idx -v
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Prints build metadata injected at compile time via `-ldflags`.
- Read-only command; no filesystem writes.

## Output

Renders an ASCII art IDX logo alongside version and build details:

```
 ██╗██████╗ ██╗  ██╗   IDX v1.2.0
 ██║██╔══██╗╚██╗██╔╝   fast BM25 code search
 ██║██║  ██║ ╚███╔╝    2024-01-15  10:30 UTC
 ██║██║  ██║ ██╔██╗    ~/my-project
 ██║██████╔╝██╔╝ ██╗
 ╚═╝╚═════╝ ╚═╝  ╚═╝
```

Fields shown alongside the logo:
- **Name + version**: `IDX <version>`
- **Tagline**: `fast BM25 code search`
- **Build date**: parsed from RFC3339 and formatted as `YYYY-MM-DD  HH:MM UTC`; falls back to the raw string if unparseable.
- **Current directory**: current working directory with home prefix collapsed to `~`.

The same output is produced by `idx --version` and `idx -v` via the Cobra version template.

## Errors

- No command-specific runtime validation errors.

## Examples

```bash
idx version
idx --version
idx -v
```
