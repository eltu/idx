# Benchmark Report: rg (run-only)

Run ID: benchmark-rg-20260511T025517Z  
Date: 2026-05-11 (UTC)
Scope: rg only (build, feature, bugfix)

## Session Metrics (Full Skill Schema)

| run_id | branch | phase | tool | started_at | finished_at | duration_seconds | tool_search_count | tool_navigation_count | context_input_tokens | context_output_tokens | context_total_tokens | token_stage_pre_build_input | token_stage_pre_build_output | token_stage_pre_build_total | token_stage_build_input | token_stage_build_output | token_stage_build_total | token_stage_feature_input | token_stage_feature_output | token_stage_feature_total | token_stage_bugfix_input | token_stage_bugfix_output | token_stage_bugfix_total | workflow_total_input_tokens | workflow_total_output_tokens | workflow_total_tokens | implementation_total_input_tokens | implementation_total_output_tokens | implementation_total_tokens | token_metrics_source | token_metrics_notes | tests_passed | notes |
|---|---|---|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|---|
| rg-build-001 | benchmark/rg-build | build | rg | 2026-05-11T02:55:51Z | 2026-05-11T02:56:19Z | 28 | 2 | 2 | 620 | 420 | 1040 | 90 | 60 | 150 | 620 | 420 | 1040 | 0 | 0 | 0 | 0 | 0 | 0 | 710 | 480 | 1190 | 620 | 420 | 1040 | estimated | Estimated from interactive transcript text volume (command + tool output) with stage time-window allocation. | yes | Build phase delivered and validated (5 tests). |
| rg-feature-001 | benchmark/rg-feature | feature | rg | 2026-05-11T02:56:36Z | 2026-05-11T02:56:54Z | 18 | 3 | 2 | 780 | 610 | 1390 | 80 | 50 | 130 | 0 | 0 | 0 | 780 | 610 | 1390 | 0 | 0 | 0 | 860 | 660 | 1520 | 780 | 610 | 1390 | estimated | Estimated from interactive transcript text volume (command + tool output) with stage time-window allocation. | yes | Feature phase delivered; one transient syntax error fixed in-phase and revalidated (6 tests). |
| rg-bugfix-001 | benchmark/rg-bugfix | bugfix | rg | 2026-05-11T02:57:01Z | 2026-05-11T02:57:20Z | 19 | 3 | 2 | 940 | 760 | 1700 | 85 | 55 | 140 | 0 | 0 | 0 | 0 | 0 | 0 | 940 | 760 | 1700 | 1025 | 815 | 1840 | 940 | 760 | 1700 | estimated | Estimated from interactive transcript text volume (command + tool output) with stage time-window allocation. | yes | bcrypt applied for new writes; list output redacted; bcrypt validation test passed (7 tests). |

## Build phase comparison (rg vs grep vs idx)

| tool | duration_seconds | tool_search_count | tool_navigation_count | context_total_tokens | tests_passed | notes |
|---|---:|---:|---:|---:|---|---|
| rg | 28 | 2 | 2 | 1040 | yes | Executed in this run |
| grep | n/a | n/a | n/a | n/a | n/a | Not executed in this scope |
| idx | n/a | n/a | n/a | n/a | n/a | Not executed in this scope |

## Feature phase comparison (rg vs grep vs idx)

| tool | duration_seconds | tool_search_count | tool_navigation_count | context_total_tokens | tests_passed | notes |
|---|---:|---:|---:|---:|---|---|
| rg | 18 | 3 | 2 | 1390 | yes | Executed in this run |
| grep | n/a | n/a | n/a | n/a | n/a | Not executed in this scope |
| idx | n/a | n/a | n/a | n/a | n/a | Not executed in this scope |

## Bugfix phase comparison (rg vs grep vs idx)

| tool | duration_seconds | tool_search_count | tool_navigation_count | context_total_tokens | tests_passed | bcrypt_validated | notes |
|---|---:|---:|---:|---:|---|---|---|
| rg | 19 | 3 | 2 | 1700 | yes | yes | Executed in this run |
| grep | n/a | n/a | n/a | n/a | n/a | n/a | Not executed in this scope |
| idx | n/a | n/a | n/a | n/a | n/a | n/a | Not executed in this scope |

## Summary

| tool | total_duration_seconds | total_tool_search_count | total_tool_navigation_count | total_context_input_tokens | total_context_output_tokens | total_context_total_tokens | total_workflow_total_input_tokens | total_workflow_total_output_tokens | total_workflow_total_tokens | total_implementation_total_input_tokens | total_implementation_total_output_tokens | total_implementation_total_tokens | overall_pass_fail_rate |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| rg | 65 | 8 | 6 | 2340 | 1790 | 4130 | 2595 | 1955 | 4550 | 2340 | 1790 | 4130 | 3/3 pass |
| grep | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| idx | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |

## Token Breakdown By Session Stage

| tool | pre_build_total_tokens | build_total_tokens | feature_total_tokens | bugfix_total_tokens | workflow_total_tokens | implementation_total_tokens |
|---|---:|---:|---:|---:|---:|---:|
| rg | 420 | 1040 | 1390 | 1700 | 4550 | 4130 |

## Methodology Notes

- Search tool scope for this run was rg only.
- Every session used interactive execution and a fresh time window ledger per phase.
- tool_search_count for rg equals direct rg invocations used for search/discovery.
- tool_navigation_count counts file context reads triggered by rg hits.
- Token provenance is estimated for all sessions because provider telemetry fields were unavailable in-session.
- Stage accounting formulas enforced:
  - token_stage_<stage>_total = input + output
  - workflow_total = pre_build + build + feature + bugfix
  - implementation_total = build + feature + bugfix
- No token total was left as zero when stage activity occurred.
- idx-specific methodology constraints are not applicable in this scoped run.

## Qualitative Observations

- rg was effective for fast literal/regex lookup during all three phases.
- Navigation overhead remained low (2 reads per phase) after targeted searches.
- A transient feature-phase syntax issue was identified quickly and corrected in-session.
- Bugfix acceptance criteria were met: new passwords hashed with bcrypt and list output no longer prints password data.

## Cleanup Confirmation

- Per-tool continuity respected: same rg sandbox reused across build -> feature -> bugfix.
- Sandbox was not cleaned between rg phases.
- After metrics capture, this run sandbox was removed.
