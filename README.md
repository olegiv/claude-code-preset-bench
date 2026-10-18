# ai-performance-cc — benchmark Claude Code presets against each other

Runs the same prompt through several Claude Code command lines ("configs"), several times each,
in a fresh workspace every time, and records per run: wall time, API time, turns, tokens, estimated
cost (split per model, so the advisor's share is visible), a deterministic pass/fail from a
`check.sh`, and an optional LLM-judge score from a `rubric.md`. Results go to `results/runs.csv`;
`summarize` turns them into tables.

Standard-library Python 3.8+ only. Needs `claude` (Claude Code CLI) and `git` on the PATH.

## Quick start

```bash
cd ai-performance-cc
python3 bench.py list        # what configs and use cases exist
python3 bench.py smoke       # one cheap 1-turn call per config + one judge call; verifies auth, flags, billing visibility
python3 bench.py run --runs 3 -u example-fix-tests     # every config x 1 use case x 3 repeats, interleaved
python3 bench.py summarize   # tables on stdout + results/summary.md
```

Useful variants:

```bash
python3 bench.py run --runs 5 -c high light            # only two configs
python3 bench.py run --dry-run                          # print the exact commands, run nothing
python3 bench.py run --no-judge                         # skip rubric grading
python3 bench.py run --judge-model opus --judge-effort high   # defaults; pick a judge that is not one of the arms
python3 bench.py run --order blocked                    # all of A, then all of B (default is interleaved: A,B,C,A,B,C)
python3 bench.py run --keep-workspaces                  # leave each run's workspace on disk for inspection
python3 bench.py summarize --since 2026-10-09T14:00     # only recent rows
```

## Layout

```
PLAN.md                       the experiment: question, arms, use-case slots, procedure, decision rule, log
install-launchers.sh          symlinks configs/*.sh onto your PATH as oclaude-high/normal/normal-max/light/light-force
bench.py                      runner, smoke test, summariser
tests/                        unittest suite, no model calls: python3 -m unittest discover -s tests
configs/
  high.sh    normal.sh    light.sh      one preset per file; mirrors your oclaude-* launchers
  normal-max.sh                         normal at --effort max: what the old oclaude-normal (ccc) ran
  light-force.sh                        light with CLAUDE_CODE_SUBAGENT_MODEL_FORCE on: every subagent on Haiku
usecases/
  example-fix-tests/                    a working example: fix two bugs so unit tests pass
    prompt.md                           frontmatter (limits, optional repo) + the prompt
    workspace/                          files copied into a fresh temp dir for every run
    check.sh                            deterministic grader (exit 0 = pass)
    rubric.md                           LLM-judge rubric (0-10)
  meowdoku/                             greenfield Go 1.27 + htmx 4 puzzle game; grading needs Go 1.27+
    README.md                           how to run it, how it is graded, the hidden tests, maintenance
    prompt.md, workspace/, rubric.md    workspace/ holds go.mod and the vendored htmx and Alpine.js
    check.sh                            gates plus hidden acceptance tests; the score is the fraction passed
  lazygit-conflict-markers/             a real fix in lazygit (upstream PR #5902); grading needs Docker
    README.md                           the source PR, the workspace, the grader, the hidden tests, the self-test
    setup.sh, pins.sh                   fetch the pinned lazygit commit into each workspace; pull the grader's image
    check.sh, hidden/, selftest.sh      grade in a container without network; hidden integration tests; known variants
    hidden/                             the hidden Go tests check.sh copies in; not given to the agent
    reference/                          local only (gitignored, not in the repository): reference solution, selftest.sh
  _template/                            copy to usecases/<name>/ ; folders starting with _ are ignored
results/
  runs.csv                              one row per run (append-only)
  summary.md                            written by `summarize`
  raw/<run_id>.json                     full Claude Code JSON result
  raw/<run_id>.result.md                agent's final message
  raw/<run_id>.diff.patch               git diff of the workspace after the run
  raw/<run_id>.stderr.txt               CLI stderr
  raw/<run_id>.judge.json               judge call result
  raw/<run_id>.check.log                full check.sh output, when the use case's check.sh writes it (meowdoku's does)
```

## Configs = launchers

A config is a shell script that `exec`s `claude` with the preset's flags and env, and passes
through whatever the harness appends. Run one with no arguments and it is simply an interactive
session with that preset, so the configs double as your daily `oclaude-*` launchers:

```bash
./install-launchers.sh          # backs up existing oclaude-* to .bak (refuses if a .bak exists), symlinks configs/*.sh in their place
```

When benchmarking, the harness appends `-p <prompt> --output-format json --no-session-persistence
--no-chrome --strict-mcp-config --mcp-config bench-mcp.json --max-turns N --max-budget-usd X`. Keep
model, effort, advisor, subagent env and `--dangerously-skip-permissions` in the config; keep
everything else out.

A run therefore sees none of the MCP servers or claude.ai connectors you have installed or
connected, and no Claude in Chrome: only the Chrome DevTools server in `bench-mcp.json`, headless,
in a throwaway profile. Your `oclaude-*` launchers are unaffected, since they do not add these
flags.

Before the first run, `run` starts each server in `bench-mcp.json` once with `--help`, so npm
fetches its package then and not inside a timed run; it needs node and npx. `run` and `smoke`
refuse to start on a machine where an administrator has deployed `managed-mcp.json`, because
Claude Code then exits at startup with `--strict-mcp-config`.

```bash
#!/bin/bash
# high: Opus main model, Fable advisor, Sonnet subagents, effort xhigh
export CLAUDE_CODE_SUBAGENT_MODEL=sonnet
exec claude --dangerously-skip-permissions --effort xhigh --model opus --advisor fable "$@"
```

Things to know:

- **`normal-max.sh` vs `normal.sh` isolates effort.** Same Opus command line; `normal` runs at
  `medium` (Opus 5.5's default), `normal-max` at `max`, which is what the old `oclaude-normal`
  (`ccc`) ran. Every config passes `--effort` explicitly, so no arm depends on the model's default.
- **Pin `--model` in every config.** Every config does; a config without it inherits your settings
  default and drifts if you or the admin change it. `smoke` warns about an unpinned config and
  shows which model actually answered.
- **`CLAUDE_CODE_SUBAGENT_MODEL` is only a default.** A subagent whose frontmatter sets `model:`
  (the pr-review-toolkit agents, for instance) ignores it, and `model: inherit` agents (built-in
  Explore and Plan) run on the main model, so in `light` Haiku only reaches
  `general-purpose` and agents with no `model:`. `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` (an on/off
  switch) puts every subagent on that model, `inherit` included; only forks and skills with
  `model: inherit` keep the main model. `light-force.sh` is `light` with it on, so that pair
  isolates the switch; `high.sh` carries it commented out.
- The harness strips `CLAUDE_CODE_EFFORT_LEVEL`, `CLAUDE_CODE_SUBAGENT_MODEL` and
  `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` from the environment before each run (they would override the
  config silently) and warns about `ANTHROPIC_MODEL`, `CLAUDE_CODE_MAX_TURNS` and
  `CLAUDE_CODE_DISABLE_ADVISOR_TOOL`. Started from inside a Claude Code session, it also drops that
  session's own variables (`SESSION_VARS`: `CLAUDECODE`, `CLAUDE_CODE_CHILD_SESSION`, its messaging
  socket and token, …), which would otherwise make every run a child of the session. It logs their
  names, never their values.
- `--advisor` and `--max-turns` are accepted in `-p` mode although `claude --help` does not list them
  (verified on 2.1.295).

## Adding a use case

Copy `usecases/_template/` to `usecases/<name>/` and edit:

**`prompt.md`** — frontmatter plus the prompt exactly as you would type it.

```
---
max_turns: 40            # default 40
max_budget_usd: 10       # default 10 (client-side estimate; subagents count; can overshoot slightly)
timeout_seconds: 1800    # harness kills the run after this
# repo: ~/Projects/my-repo
---
Add a `/healthz` endpoint that returns 200 and the git SHA. Add a test. Do not touch CI config.
```

**Starting state** — two options. With `repo:` set, every run gets a fresh detached
`git worktree` of that repo's `HEAD` (uncommitted changes are not included, untracked
`.claude/settings.local.json` is not included either). Without `repo:`, the contents of
`workspace/` are copied into a fresh temp dir and committed as a `base` commit, so the diff can be
captured afterwards; if there is no `workspace/`, the run starts in an empty git repo.

**`setup.sh`** (optional) — runs inside the workspace before the agent, e.g. `npm ci`.

**`check.sh`** (optional but recommended) — the deterministic grader. Runs inside the workspace
after the agent. Exit 0 = pass. If the last stdout line is a number between 0 and 1 it is recorded
as a partial score. Environment: `WORKSPACE`, `USECASE_DIR`, `RUN_ID`, `CONFIG`, `RESULT_JSON`
(path to the full result), `RESULT_TEXT` (path to the final message). Typical checks: the test
suite passes, the build compiles, a file exists, a file you told the agent not to touch is
byte-identical to the original (`cmp -s`).

**`rubric.md`** (optional) — what a 10 looks like, what loses points, what is a 0. The judge sees
the rubric, the prompt, the agent's final message, the workspace diff (truncated at 120k chars) and
the `check.sh` verdict, and returns a 0-10 score via structured output. The judge runs with
`--tools ""`, `--no-chrome` and `--strict-mcp-config`, so it has no tools and only reads what it is
given.

For use cases whose output is not code (a review, a document), use `rubric.md` on its own, or a
`check.sh` that only verifies structural things (file exists, has the expected sections).

## What is measured

Each row in `results/runs.csv`:

| column | meaning |
|---|---|
| `subtype` | `success`, `error_max_turns`, `error_max_budget_usd`, `error_during_execution`, or harness values `harness_timeout`, `no_json`, `harness_error` |
| `wall_s` | wall clock measured by the harness around the whole CLI process |
| `duration_ms`, `duration_api_ms` | Claude Code's own run time and time spent waiting on the API |
| `num_turns` | assistant turns |
| `total_cost_usd` | Claude Code's client-side estimate from its price table (not billing); covers all models incl. subagents and advisor |
| `advisor_cost_usd` | the part of `total_cost_usd` billed to the config's advisor model |
| `per_model_cost_usd` | JSON: cost per model id |
| `main_model`, `models_used` | which models actually answered (full ids from `modelUsage`) |
| `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `thinking_tokens` | main-loop usage (subagents excluded here, included in `per_model_cost_usd`) |
| `subagent_stats` | JSON from Claude Code: spawned/completed/failed subagents |
| `permission_denials` | non-zero means the config is missing `--dangerously-skip-permissions` |
| `files_changed` | files added/modified in the workspace |
| `check_pass`, `check_score` | from `check.sh` |
| `judge_score`, `judge_model`, `judge_cost_usd` | from the rubric judge; judge cost is kept separate and never added to the run's cost |
| `notes` | timeouts, missing JSON, "advisor not in modelUsage", harness errors |

`summarize` prints medians per (use case, config) and per config pooled, plus which models were
billed per config and every run that has a note.

## Methodology notes

- **Repeat.** Agent runs are non-deterministic; one run per config is noise. Use `--runs 3` at
  least, 5 if the arms look close. Compare medians and look at min-max.
- **Interleave.** The default order is A,B,C,A,B,C so API latency drift and prompt-cache warmth are
  spread across arms instead of favouring whichever ran last. `--shuffle` randomises within each
  repeat; `--order blocked` is there if you specifically want warm-cache numbers.
- **Grade.** Without `check.sh` or `rubric.md`, the cheapest arm wins by construction. Pass rate x
  cost x time is the comparison that answers "which preset should I use for this kind of task".
- **Advisor cost is visible.** In a real run the advisor model shows up in `modelUsage` with its own
  `costUSD` (on a small fix with the `light` preset, the Opus advisor cost more than the Sonnet
  main model). On a 1-turn smoke run the advisor is usually never called, so `smoke` reports that as
  a note, not an error. If a *real* run of an advisor config never lists the advisor in
  `models_used`, treat its cost as under-reported.
- **Cost is an estimate.** `total_cost_usd` comes from Claude Code's bundled price list. On an
  Enterprise/subscription seat it is a proxy for plan usage, not an invoice. It is consistent across
  arms, which is what matters here.
- **Caps.** `max_turns` and `max_budget_usd` stop runaway arms; a capped run is recorded with
  `subtype=error_max_turns` / `error_max_budget_usd` and counts as a failure.
- **Same starting state.** Every run gets a fresh workspace; nothing from a previous run leaks.
  Claude Code still reads your user-level settings, `~/.claude/CLAUDE.md`, plugins and hooks, which
  is what a real session sees. If you want a sterile run, add `--bare` to a config — but note it
  also skips CLAUDE.md auto-discovery, hooks and plugins, so only do it in every arm or none.
- **Judge outside the arms.** Default judge is `opus` at effort `high`. If Opus is also one of your
  main models the harness only notes it; a cleaner choice is a model that is not under test, kept
  identical for the whole experiment.
- **Budget the experiment.** 3 configs x 3 use cases x 5 repeats = 45 agent runs plus 45 judge
  calls. Run `--dry-run` first, then one repeat (`--runs 1`) to see per-run cost before scaling up.

## Tests

```bash
python3 -m unittest discover -s tests -v
```

Standard-library `unittest`, and no model calls: a fake `claude` stands in for the CLI, and
workspaces and the launcher installer run in temporary directories. CI runs the suite on Python
3.8 and 3.14, and lints `bench.py` and the tests with pylint.

## Known limits

- One run at a time; no concurrency (keeps timing comparable and avoids rate-limit effects).
- `check.sh` runs on your machine with your tools; it is not sandboxed.
- The agent is not sandboxed either: an arm runs with `--dangerously-skip-permissions`, as you,
  with your files and network, so it can read or change this checkout, a use case's `check.sh`
  and hidden tests included. Only the workspace and the prompt are given to it.
- The judge's view of the diff is truncated at 120k characters and of the final message at 12k.
- `repo:` worktrees start at `HEAD`; if the task needs uncommitted scaffolding, commit it on a
  branch first or put it in `workspace/` instead.
