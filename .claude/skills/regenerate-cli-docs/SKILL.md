---
name: regenerate-cli-docs
description: 'Regenerate end-user CLI documentation from source code. Use for updating docs/features from Cobra commands, flags, defaults, validations, errors, and daemon/watch/search behavior. Produces per-feature Markdown files in docs/features with contracts and examples.'
argument-hint: 'Scope to document, for example: all commands or only search/daemon/watch'
user-invocable: true
---

# Regenerate CLI Docs

## What This Skill Produces
This skill regenerates user-facing CLI documentation from the current codebase and writes detailed Markdown files under docs/features.

Output style:
- English only
- One file per command or feature area
- Explicit contracts: inputs, flags, defaults, validation rules, outputs, errors, and side effects
- Consistent with current docs/features structure

## When to Use
Use this skill when:
- CLI flags or command behavior changed
- Validation or error messages changed
- You added commands/subcommands/options
- You want to refresh all command docs from code truth

Trigger phrases:
- regenerate CLI docs
- update docs/features
- document all commands
- refresh command contracts

## Inputs
- Optional scope argument, for example:
  - all commands
  - search only
  - daemon and watch

## Procedure
1. Discover command surface
- Read Cobra wiring and command builders.
- Enumerate all commands, subcommands, and aliases.
- Capture usage strings and command purpose.

2. Extract runtime contracts from code
- For each command, extract:
  - positional args contract
  - flags, defaults, types, repeatability
  - validation rules and failure cases
  - execution path (service methods called)
  - notable side effects (filesystem writes, daemon state, watcher behavior)

3. Extract user-visible output and errors
- Identify success messages, warnings, and failure messages.
- Distinguish user output from internal logs.
- Document exact behavior for empty/no-result cases and pagination.

4. Map features to docs/files
- Keep current structure in docs/features.
- Update the command-specific file if it exists.
- Create a new feature file only when a command/feature has no existing file.
- Update docs/features/README.md as the index of feature docs.

5. Write docs by functionality
- Group by command/feature area (init, sync, search, inspect, watch, daemon, destroy, errors).
- Include short examples for common and advanced usage.
- Include a clear table for flags and defaults.

6. Consistency pass
- Ensure terminology and examples are consistent across files.
- Remove stale flags/options/examples that no longer exist.
- Keep wording concise and actionable for end users.

7. Verification pass
- Re-check every documented flag/arg against current code.
- Confirm documented errors still match current validations.
- Confirm all docs are English only.

## Decision Rules
- If code and docs disagree, code is the source of truth.
- If behavior is implicit but stable, document it as Notes.
- If behavior is ambiguous, document the observed behavior and list an open question.
- If scope argument is partial, update only related docs and preserve unrelated files.

## Required Sections Per Command File
- Purpose
- Usage
- Arguments
- Flags
- Behavior and Side Effects
- Output
- Errors
- Examples

## Quality Checks
- Every command has at least one example.
- Every flag in code appears in docs with correct default.
- No non-English text remains.
- No monolithic single file replacing docs/features split.
- docs/features/README.md links to all maintained feature files.

## Completion Criteria
This skill is complete when:
- docs/features reflects current CLI behavior end-to-end
- contracts and parameters are fully documented by functionality
- outputs/errors are clear for end users
- the docs are consistent, English-only, and easy to navigate
