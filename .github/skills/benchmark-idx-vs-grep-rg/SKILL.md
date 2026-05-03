---
name: benchmark-idx-vs-grep-rg
description: "Run a controlled coding benchmark across idx, grep, and rg with context reset between phases; track session duration and per-tool interaction counts; compare outcomes for build, feature, and bugfix tasks. Use when: benchmark idx vs grep vs rg, compare search approaches, measure coding session efficiency."
argument-hint: "Scope to run: full benchmark or a specific phase (build, feature, bugfix)"
user-invocable: true
---

# Benchmark idx vs grep vs rg

## What This Skill Produces
This skill runs a repeatable benchmark that compares three search approaches during software delivery work:
- idx
- grep
- rg

For each approach, the benchmark measures:
- start and end time per session
- total session duration
- count of interactions that used the session's target tool
- delivery correctness across three phases

The benchmark outputs a final comparison report with metrics and observations.

## When to Use
Use this skill when:
- You want an apples-to-apples comparison between idx, grep, and rg.
- You want to measure speed and interaction volume for coding tasks.
- You need a reproducible workflow with context resets between tasks.

Trigger phrases:
- benchmark idx vs grep vs rg
- compare idx and ripgrep workflow
- run search tool efficiency benchmark
- benchmark coding sessions by search approach

## Inputs
Optional scope argument:
- full benchmark
- build only
- feature only
- bugfix only

Optional implementation constraints:
- language/runtime version
- storage format
- CLI UX expectations

## Mandatory Workspace (Sandbox)
All benchmark implementation files must be created inside this project path:

`./.claude/skills/benchmark-idx-vs-grep-rg/sandbox/`

Use a per-run working directory inside sandbox (for example `sandbox/run-003/studentreg`).
Do not create benchmark target projects under `/tmp` or outside this repository.

Rationale:
- `idx` indexing and search in this workflow must discover files from inside the project workspace.

Mandatory cleanup after benchmark completion:
- Remove all generated files and folders under `sandbox/`.
- Keep only intentional placeholders (for example `.gitkeep`), if present.
- Confirm sandbox is empty (or placeholder-only) before finishing the skill.

## Benchmark Workload
The same workload must be implemented in all three approaches.

Phase 1: Base solution
- Build a Go CLI for student registration.
- Support create/insert only.
- Required fields include personal data plus login and password.

Phase 2: New functionality
- Add a command to list all registered students.
- Initially list all stored fields.

Phase 3: Bug fix
- Fix plaintext password exposure.
- Store password using bcrypt.
- Ensure list output does not expose the plaintext password.
- Validate the fix by confirming stored value is a valid bcrypt hash and the list command never outputs the raw password.

## Branch Strategy
Create 9 short-lived local branches, one per phase per tool:
- benchmark/idx-build
- benchmark/idx-feature
- benchmark/idx-bugfix
- benchmark/grep-build
- benchmark/grep-feature
- benchmark/grep-bugfix
- benchmark/rg-build
- benchmark/rg-feature
- benchmark/rg-bugfix

Each branch contains only the changes for that phase and tool combination.
Do not reuse implementation artifacts across branches.
After the session metrics are recorded for a branch, delete the branch locally.
Branches exist only for statistical traceability during the session.

## idx Setup and Usage Reference

The benchmark sessions that use idx must follow the exact commands below.
This project requires Go 1.26+ and a Git repository root.
The AGENTS.md of this project forbids using grep or rg inside the project itself,
but the benchmark target project (student registration CLI) is a separate codebase
where the session tool restriction applies — follow only the tool rule for that session.

Build idx before starting any idx session:

```bash
make build
```

This produces the binary at `./bin/idx`.
Use the binary for all idx commands during the session:

```bash
./bin/idx init
./bin/idx search "<terms>"
./bin/idx search "<terms>" --files-only
./bin/idx search "<terms>" --path <path-filter>
./bin/idx search --path <path-filter>
./bin/idx search "<terms>" --format json --matches-only
./bin/idx search "<terms>" --context <lines>
./bin/idx sync
./bin/idx status
```

Alternatively, run without building:

```bash
go run cmd/idx/main.go search "<terms>"
go run cmd/idx/main.go search "<terms>" --files-only
go run cmd/idx/main.go search "<terms>" --path <path-filter>
go run cmd/idx/main.go search --path <path-filter>
go run cmd/idx/main.go search "<terms>" --format json --matches-only
go run cmd/idx/main.go search "<terms>" --context <lines>
```

idx must be initialized before searching. Run init once in the target project root:

```bash
./bin/idx init
# or
go run cmd/idx/main.go init
```

After filesystem changes, resync:

```bash
./bin/idx sync
# or
go run cmd/idx/main.go sync
```

Before starting each idx benchmark session, verify daemon state in the target project root:

```bash
./bin/idx daemon status
```

If daemon monitoring is not active for the project, initialize it before continuing:

```bash
./bin/idx daemon enable .
./bin/idx daemon status
```

Count a `tool_search_count` interaction for every `search` invocation.
Count a `tool_navigation_count` interaction for every file opened as a direct result of a search hit.

## Session Isolation Rules
For every phase in every branch:
1. Start a fresh conversation context before beginning the phase.
2. Do not reuse previous chat history for that phase.
3. Execute only the session's target search approach:
- idx branch: use idx search commands only.
- grep branch: use grep only.
- rg branch: use rg only.
4. Capture timestamps and interaction counts during that session.

For every new invocation of this skill (full or partial scope):
1. Treat the execution as a first-time run.
2. Ignore prior benchmark interaction context, prior chat-derived optimizations, and prior tool usage patterns.
3. Recreate the benchmark flow from scratch instead of continuing from previous session state.
4. Prioritize simulation fidelity over metric outcomes; metrics are directional signals, not optimization targets.
5. Do not use previous run metrics to influence implementation decisions during the current run.

## Agent Interactive Execution Mode (Mandatory)
This benchmark must be executed in interactive agent mode, step by step.

Required:
- Execute commands and edits interactively, one step at a time, as part of the live session flow.
- Keep per-step traceability so each interaction can be counted and audited.
- Treat every command relevant to the session timeline as part of the simulation record.

Forbidden:
- Creating or using automation scripts (for example .sh files) to batch benchmark phases.
- Running bundled command blocks that skip observable intermediate steps.
- Replaying previously generated automation artifacts to fast-forward sessions.

Invalidation rule:
- If script-based or batched automation is used for a session, mark that session invalid and rerun it in interactive mode.

Total sessions in full benchmark:
- 9 sessions (3 branches x 3 phases)

## Per-Session Procedure
1. Record metadata
- branch name
- phase name
- session tool
- session start timestamp

2. Implement the required phase changes
- keep scope limited to the current phase
- add tests for new behavior and regressions when applicable

3. Validate
- run project tests
- run manual CLI checks for phase behavior

4. Record completion data
- session end timestamp
- session duration
- number of interactions using the session tool
- result status (pass/fail)

## Metrics Logging Format
Use one row per session with this schema:
- run_id
- branch
- phase (build | feature | bugfix)
- tool (idx | grep | rg)
- started_at
- finished_at
- duration_seconds
- tool_search_count
- tool_navigation_count
- tests_passed
- notes

Group the final comparison report by problem (phase), not by tool, so each phase produces an independent side-by-side comparison across the three tools.

Recommended run_id format:
- idx-build-001
- idx-feature-001
- idx-bugfix-001
- grep-build-001
- grep-feature-001
- grep-bugfix-001
- rg-build-001
- rg-feature-001
- rg-bugfix-001

Per-session schema must include both interaction counters:
- tool_search_count (direct search command invocations)
- tool_navigation_count (file reads/jumps triggered by search results)

## Tool Interaction Counting Rules
Count two categories of interactions separately.

Search interactions (direct tool use):
- idx session: increment this counter only for `idx search` (or `go run cmd/idx/main.go search`) invocations
- grep session: each grep command increments this counter
- rg session: each rg command increments this counter

Navigation interactions (tool-driven exploration):
- File reads or directory listings triggered as a direct result of a search hit
- Symbol lookups or go-to-definition jumps initiated because of a search result
- Opening a file to read context lines around a match

Record both counts per session:
- tool_search_count
- tool_navigation_count

Do not count:
- idx commands other than search (`idx init`, `idx sync`, `idx status`, `idx daemon ...`)
- git commands
- test commands
- formatter or lint commands
- file reads with no relationship to a search result

## Decision Rules
- If behavior differs across branches, prefer correctness first, then speed.
- If a phase fails validation, mark run as fail and record root cause.
- If scope argument is partial, execute only requested phases.
- If any branch changes requirements, restart that phase in all branches.
- For a new skill invocation, reset execution assumptions and avoid carrying over prior interaction memory to reduce benchmark bias.
- If execution mode is not interactive agent mode, discard the run and restart in interactive mode.

## Quality Checks
- Same workload and acceptance criteria across all branches.
- Same phase order across all branches.
- Fresh context before every phase.
- One target search tool per session.
- Interactive agent execution mode used end-to-end (no scripts, no batch shortcuts).
- Benchmark target project lives under `./.claude/skills/benchmark-idx-vs-grep-rg/sandbox/`.
- Every idx session runs daemon pre-check (`idx daemon status`) before starting implementation.
- If daemon is not active for the project, initialize with `idx daemon enable .` before continuing idx session work.
- Start/end timestamps recorded for every session.
- Both tool_search_count and tool_navigation_count recorded for every session.
- Tests executed and status recorded for every session.
- bcrypt usage validated in all three bugfix sessions.
- Branch deleted after session metrics are captured.
- Sandbox cleaned after benchmark completion.

## Deliverables
- Code delivered on each branch during its session.
- Branch deleted after session metrics are recorded.
- Session metrics table for all executed runs.
- Final comparison report written as a Markdown file at:

  `docs/benchmarks/idx-vs-grep-rg.md`

  The file must contain the following sections:

  ### Build phase comparison (idx vs grep vs rg)
  Table with columns: tool, duration_seconds, tool_search_count, tool_navigation_count, tests_passed, notes

  ### Feature phase comparison (idx vs grep vs rg)
  Same table structure as build phase.

  ### Bugfix phase comparison (idx vs grep vs rg)
  Same table structure plus a column: bcrypt_validated (yes/no)

  ### Summary
  - Total duration per tool (sum across all phases)
  - Total tool_search_count per tool
  - Total tool_navigation_count per tool
  - Overall pass/fail rate per tool
- Methodology note: for idx sessions, `tool_search_count` includes only `idx search` (or `go run cmd/idx/main.go search`) invocations.
- Methodology note: for idx sessions, daemon state is checked before each session and initialized when missing.
  - Qualitative observations and highlights

  If the file already exists, append or update the relevant sections without removing prior benchmark runs.

- Post-run cleanup confirmation:
  - `./.claude/skills/benchmark-idx-vs-grep-rg/sandbox/` is empty (or placeholder-only).

## Completion Criteria
This skill is complete when:
- All requested phases are executed for each selected branch.
- Every session has timing and interaction metrics captured.
- All phase outcomes are validated.
- A final comparison report is produced.

## Example Prompts
- Run a full benchmark with idx vs grep vs rg and produce the final metrics table.
- Run only the bugfix phase benchmark for all three branches.
- Benchmark build and feature phases only, then summarize speed and interaction counts.
