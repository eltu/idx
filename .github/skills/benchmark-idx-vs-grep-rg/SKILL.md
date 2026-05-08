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
- context consumed per scenario (from reset baseline)
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
All benchmark implementation files must be created inside a temporary directory:

`/tmp/idx-benchmark/<run-id>/`

Use a per-run working directory inside that path (for example `/tmp/idx-benchmark/run-003/studentreg`).
Do not create benchmark target projects inside this repository.

### Phase continuity within a tool session
The three phases for each tool (build → feature → bugfix) are sequential and share the same codebase:
- **build** creates the project from scratch.
- **feature** adds functionality on top of the build artifacts.
- **bugfix** fixes a problem in the project that already has the feature.

Do **not** delete or reset the sandbox directory between phases of the same tool.
The same directory must be carried forward so each phase starts from the state left by the previous one.

Sandbox cleanup timing:
- Clean the sandbox for a tool only **after** its bugfix phase is fully complete and metrics are recorded.
- Never clean between phases (build → feature or feature → bugfix) of the same tool.
- After all tool sessions (or after the last requested phase of the last tool), remove all generated files and folders under `/tmp/idx-benchmark/`.
- Confirm sandbox is empty (or removed) before finishing the skill.

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
Phases within the same tool are sequential: the feature branch extends the build, and the bugfix branch extends the feature.
Do not reuse sandbox artifacts across different tools (idx sandbox is independent from grep sandbox).
After all three phases for a tool are complete and metrics are recorded, delete those three branches locally.
Branches exist only for statistical traceability during the session.

## idx Setup and Usage Reference

The benchmark sessions that use idx must follow the exact commands below.
The AGENTS.md of this project forbids using grep or rg inside the project itself,
but the benchmark target project (student registration CLI) is a separate codebase
where the session tool restriction applies — follow only the tool rule for that session.

The `idx` binary is installed at `~/.local/bin/idx` and available globally in the shell.
Use the `idx` command directly for all idx operations during the session.

Set this path once per session:

```bash
TARGET_ROOT=/tmp/idx-benchmark/<run-id>/<tool-phase>/studentreg
```

Use the command patterns below. Commands that must operate on the benchmark target project
use a subshell that changes to `TARGET_ROOT`:

All `idx search` invocations in benchmark sessions **must** include `--agent-compact`
to reduce context/token usage consistently across idx scenarios.
`--format json` is **not required** for idx benchmark runs; prefer default text output with `--agent-compact`.
For idx **feature** and **bugfix** phases, use an `--operator OR` query as the first attempt to minimize `tool_search_count`.
Open a second idx search only if the first OR query returns no useful hits for implementation.

```bash
(cd "$TARGET_ROOT" && idx search "<terms>" --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --files-only --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --matches-only --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --path <path-filter> --agent-compact)
(cd "$TARGET_ROOT" && idx search --path <path-filter> --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --ext <extension> --agent-compact)
(cd "$TARGET_ROOT" && idx search --path <path-filter> --ext <extension> --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --explain --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --context <lines> --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --from <offset> --size <limit> --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --operator OR --agent-compact)
(cd "$TARGET_ROOT" && idx search "<terms>" --operator AND --relaxation '>N' --agent-compact)
(cd "$TARGET_ROOT" && idx sync --quiet)
```

Before testing idx and before starting the session timer, run the single pre-step:

```bash
idx daemon enable "$TARGET_ROOT" --quiet
```

`daemon enable` is idempotent: it auto-inits the index when missing and exits 0 if the daemon is already running.
`--quiet` suppresses informational confirmations from entering the agent context window (errors still go to stderr).
No separate `idx init` or `idx daemon status` call is needed.

After filesystem changes, resync:

```bash
(cd "$TARGET_ROOT" && idx sync --quiet)
```

Count a `tool_search_count` interaction for every `search` invocation.
Count a `tool_navigation_count` interaction for every file opened as a direct result of a search hit.
For idx sessions, this counting rule applies to `idx search` calls that include `--agent-compact`.

## Session Isolation Rules
Scenario definition for this skill:
- One scenario = one tool + one phase session (for example: idx-build, grep-feature, rg-bugfix).

For every phase in every branch:
1. Start a fresh conversation context before beginning the phase.
2. Do not reuse previous chat history for that phase.
3. Execute only the session's target search approach:
- idx branch: use idx search commands only.
- grep branch: use grep only.
- rg branch: use rg only.
4. Capture timestamps, interaction counts, and context consumption during that session.
5. If context was not reset before the scenario, invalidate the scenario and rerun it with a fresh context.

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

Pre-step before recording session start timestamp:
- Run `idx daemon enable "$TARGET_ROOT" --quiet` (idempotent: auto-inits if needed, exits 0 when already monitoring; `--quiet` keeps confirmations out of context).
- Only start timing after the command completes.

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
- context usage metrics for the scenario (prompt/input, completion/output, and total when available)
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
- context_input_tokens
- context_output_tokens
- context_total_tokens
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

Per-session schema must include context counters per scenario:
- context_input_tokens
- context_output_tokens
- context_total_tokens

## Tool Interaction Counting Rules
Count two categories of interactions separately.

Search interactions (direct tool use):
- idx session: increment this counter only for `idx search` invocations
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
- idx commands other than search (`idx init`, `idx sync`, `idx status`, `idx daemon ...`, `idx version`)
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
- For idx feature/bugfix phases, the first search must be `idx search "<termA> <termB>" --operator OR --agent-compact`.
- If a second idx search is needed after the OR-first attempt, record the reason in session notes (e.g., "OR-first had no useful hits").

## Quality Checks
- Same workload and acceptance criteria across all branches.
- Same phase order across all branches.
- Fresh context before every phase.
- One target search tool per session.
- Interactive agent execution mode used end-to-end (no scripts, no batch shortcuts).
- Benchmark target project lives under `/tmp/idx-benchmark/`.
- Every idx session runs `idx daemon enable "$TARGET_ROOT" --quiet` as the sole pre-step before starting session timing (idempotent: handles init and daemon start in one command).
- All idx session commands use the `idx` binary available in the shell (`~/.local/bin/idx`).
- All idx session `search` commands include `--agent-compact` to minimize context usage.
- In idx feature/bugfix sessions, first-search strategy uses `--operator OR`; extra idx searches are justified in notes.
- Start/end timestamps recorded for every session.
- Both tool_search_count and tool_navigation_count recorded for every session.
- Context reset performed for every scenario before measurement starts.
- Context token metrics (input/output/total) recorded for every session.
- Tests executed and status recorded for every session.
- bcrypt usage validated in all three bugfix sessions.
- Branches for a tool deleted only after all three phases of that tool are complete and metrics are recorded.
- Sandbox directory for a tool deleted only after its bugfix phase is complete and metrics are recorded.
- Sandbox never cleaned between phases of the same tool (build → feature → bugfix share artifacts).
- Sandbox fully empty (or placeholder-only) confirmed before finishing the skill.

## Deliverables
- Code delivered on each branch during its session.
- Branch deleted after session metrics are recorded.
- Session metrics table for all executed runs.
- Final comparison report written as a Markdown file at:

  `docs/benchmarks/idx-vs-grep-rg.md`

  The file must contain the following sections:

  ### Build phase comparison (idx vs grep vs rg)
  Table with columns: tool, duration_seconds, tool_search_count, tool_navigation_count, context_total_tokens, tests_passed, notes

  ### Feature phase comparison (idx vs grep vs rg)
  Same table structure as build phase.

  ### Bugfix phase comparison (idx vs grep vs rg)
  Same table structure plus a column: bcrypt_validated (yes/no)

  ### Summary
  - Total duration per tool (sum across all phases)
  - Total tool_search_count per tool
  - Total tool_navigation_count per tool
  - Total context_input_tokens per tool
  - Total context_output_tokens per tool
  - Total context_total_tokens per tool
  - Overall pass/fail rate per tool
- Methodology note: for idx sessions, `tool_search_count` includes only `idx search` invocations.
  - Methodology note: for idx sessions, `idx daemon enable "$TARGET_ROOT" --quiet` is the sole pre-step (idempotent: handles init + daemon start). No separate `idx init` or `idx daemon status` call is made before timing.
  - Methodology note: for idx sessions, every `idx search` invocation includes `--agent-compact` to enforce compact output and reduce token/context overhead.
  - Methodology note: for idx feature/bugfix sessions, first query uses `--operator OR`; additional idx searches are allowed only with documented reason.
  - Methodology note: each scenario starts from a fresh context; context token counters must be measured only after this reset and never carried across scenarios.
  - Qualitative observations and highlights

  If the file already exists, append or update the relevant sections without removing prior benchmark runs.

- Post-run cleanup confirmation:
  - Sandbox cleaned per-tool: each tool's sandbox directory is removed only after its bugfix phase completes.
  - Never cleaned between phases of the same tool — build → feature → bugfix share the same sandbox.
  - `/tmp/idx-benchmark/` is removed after all tool sessions finish.

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
