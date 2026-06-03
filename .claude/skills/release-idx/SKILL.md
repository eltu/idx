---
name: release-idx
description: >
  Cut a new idx release end-to-end: audit docs, run the full test suite,
  determine the next semver, write release notes, commit, tag, and push.
  Use when asked to "release idx", "cut a release", "bump version", or
  "create a new release".
---

# release-idx

End-to-end release workflow for idx. Covers five phases in order — stop
at any gap and resolve it before continuing.

All commands run from the **repo root**.

---

## Phase 1 — Doc audit

Run the audit script to check feature docs, ADR files, and the releases index:

```bash
bash .claude/skills/release-idx/check-docs.sh
```

**If it exits 1:** fix the gaps before continuing.

Common gaps:
- New command shipped but `docs/features/<command>.md` not created → regenerate with `/regenerate-cli-docs`.
- New ADR recorded in `CLAUDE.md` but file missing in `docs/adr/` → create the ADR file.
- Release note file exists but not listed in `docs/releases/index.md` → add the row.

---

## Phase 2 — Full test suite

```bash
# Format + lint + unit tests (same gate as pre-push hook)
make check

# Coverage gate — must stay at or above 89%
go test -coverprofile=coverage.out ./... && \
  go tool cover -func=coverage.out | grep total

# End-to-end tests (binary must be built first)
make build
go test ./cmd/idx/... -v -count=1 2>&1 | tail -30

# Concurrency / race tests
make test-concurrency-race
```

Stop if any suite fails. Fix, then re-run from the top of this phase.

---

## Phase 3 — Next version

Determine the bump from conventional commits since the last tag:

```bash
bash .claude/skills/release-idx/next-version.sh
```

**Bump policy:**
| Commit prefix | Bump |
|---|---|
| `feat!:` / `BREAKING CHANGE` | major |
| `feat(…):` | minor |
| `fix:` / `chore:` / `docs:` / `refactor:` | patch |

The script prints the next version (e.g. `v0.8.0`). Use that value in the
steps below as `<VERSION>`.

**Sanity-check:** list the commits that will be included:

```bash
LAST_TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]' | head -1)
git log "${LAST_TAG}..HEAD" --oneline
```

---

## Phase 4 — Release notes

### 4a. Create `docs/releases/<VERSION>.md`

Follow the format of `docs/releases/v0.7.0.md` exactly:

```markdown
# <VERSION>

**Released:** <Month D, YYYY> · [Download](https://github.com/eltu/idx/releases/tag/<VERSION>)

---

## ✨ New features

### <Feature title>
<Description>

---

## 🐛 Bug fixes / Improvements

### <Fix title>
<Description>

---

## 🏗️ Architecture

- ADR XXXX — <Decision summary>.
```

Rules:
- Group `feat:` commits under **New features**.
- Group `fix:` under **Bug fixes / Improvements**.
- Group `refactor:` / architecture decisions under **Architecture**.
- Omit `chore:`, `docs:`, `test:` commits — they don't belong in release notes.
- Write English. Be user-facing, not commit-message-style.
- Include code examples for any new flags or commands.

### 4b. Update `docs/releases/index.md`

Add a new row at the top of the table:

```markdown
| [<VERSION>](<VERSION>.md) | <Month D, YYYY> | <One-line highlights> |
```

### 4c. Re-run the doc audit

```bash
bash .claude/skills/release-idx/check-docs.sh
```

Must exit 0 before continuing.

---

## Phase 5 — Commit, tag, push

```bash
# Stage only docs changes
git add docs/releases/

# Commit
git commit -m "docs: add <VERSION> release notes"

# Push to main
git push origin main

# Delete old tag if it exists (only when re-releasing same version)
# git tag -d <VERSION> && git push origin :refs/tags/<VERSION>

# Create annotated tag with release notes summary
git tag -a <VERSION> -m "<paste release notes summary here>"

# Push the tag — triggers GoReleaser CI
git push origin <VERSION>
```

**After pushing the tag**, monitor the Release workflow on GitHub Actions:
- `make check` runs on the CI runner.
- GoReleaser builds linux/darwin amd64/arm64 binaries and uploads them.
- If the run fails due to existing assets (`422 already_exists`): delete the
  GitHub release (not the tag) with `gh release delete <VERSION> --yes`,
  then delete and recreate the tag.

---

## Gotchas

- **`printf '%04d'` and leading-zero ADR numbers**: ADR numbers like `0008` are
  treated as invalid octal by bash `printf`. `check-docs.sh` strips leading
  zeros before formatting — don't copy-paste `printf '%04d'` on raw ADR strings.
- **Race condition in TUI tests**: tests that mutate `inspectAvailableCommands`
  must not use `t.Parallel()`. If `make check` flakes on `internal/app/tui`,
  that's the cause.
- **GoReleaser `422 already_exists`**: the GitHub release exists from a prior
  run. Delete the release with `gh release delete <VERSION> --yes`, then
  recreate the tag. `.goreleaser.yml` has `replace_existing_artifacts: true`
  to handle this automatically from v0.7.0 onwards.
- **`IDX_SITE_TOKEN` missing**: the `docs-deploy.yml` workflow dispatches to
  `eltu/idx-site` using this secret. If it's not set in repo secrets, the
  dispatch step fails silently (GoReleaser still succeeds).
- **Tag protection rules**: GitHub may warn `Cannot create ref due to creations
  being restricted` for tag pushes — this is bypassed and the push still lands.
- **`next-version.sh` returns `patch` for post-release commits**: after a
  release the next tag bump defaults to `patch` unless there are `feat:` commits
  in the window. Always review the commit list with `git log` to confirm.
