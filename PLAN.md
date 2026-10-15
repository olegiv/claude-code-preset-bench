# Benchmark plan — which Claude Code preset for which kind of task?

Status: **design agreed, use cases to be defined** (2026-10-09). Update the log at the bottom as runs happen.

## Question

For three representative work tasks, which of five Claude Code presets gives the best outcome per
unit of cost and time? In particular: does the "Sonnet builds, Haiku explores, Opus reviews" recipe
(X post by @_guillecasaus, 2026-10) actually save Opus spend without losing quality, compared with
plain Opus and with Opus plus a Fable advisor?

## Arms (configs/)

| arm | main model | advisor | subagent default | effort | command line |
|---|---|---|---|---|---|
| `high` | Opus 5.5 | Fable | Sonnet | xhigh | `CLAUDE_CODE_SUBAGENT_MODEL=sonnet claude --effort xhigh --model opus --advisor fable` |
| `normal` | Opus 5.5 | — | (settings) | medium | `claude --effort medium --model opus` |
| `normal-max` | Opus 5.5 | — | (settings) | max | `claude --effort max --model opus` |
| `light` | Sonnet 5.5 | Opus | Haiku | medium | `CLAUDE_CODE_SUBAGENT_MODEL=haiku claude --effort medium --model sonnet --advisor opus` |
| `light-force` | Sonnet 5.5 | Opus | Haiku, forced | medium | `CLAUDE_CODE_SUBAGENT_MODEL=haiku CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1 claude --effort medium --model sonnet --advisor opus` |

All arms add `--dangerously-skip-permissions`; the harness adds `-p --output-format json
--no-session-persistence --max-turns N --max-budget-usd X`.

Design notes:

- `normal` is pinned to Opus on the assumption that this was the settings default. If the real
  default was Sonnet, change the one word in `configs/normal.sh` and note it in the log.
- `normal` vs `normal-max` differ only in effort (medium vs max), so that pair shows whether max
  buys anything on Opus. `light` vs `light-force` differ only in `CLAUDE_CODE_SUBAGENT_MODEL_FORCE`.
  `normal` vs `light`/`light-force` is the headline comparison: plain Opus vs the recipe, both at
  default effort, without and with Haiku actually running Explore and Plan. `high` differs from
  `normal` in advisor, subagent model and effort (xhigh): it is the hard-task preset, not an
  isolating pair.
- `CLAUDE_CODE_SUBAGENT_MODEL` is a *default*. Subagents with `model:` in their frontmatter (the
  pr-review-toolkit agents) keep their own model, and `model: inherit` agents (built-in Explore and
  Plan) run on the main model. This is deliberate: the arms are measured as they behave in real use.
  `light-force` is the stricter arm: `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` is an on/off switch that
  makes every subagent ignore its own `model:`, `inherit` included; only forks and skills with
  `model: inherit` keep the main model. `high.sh` carries it commented out.
- The post's `--subagents haiku` flag does not exist (`unknown option` on 2.1.295); the env var is
  the correct mechanism and is what the launchers already used.

## Use cases (usecases/) — to be defined

Three tasks, each with a deterministic `check.sh` where possible and a `rubric.md` for the judge.
The post's claimed benefits only appear in tasks that have real exploration and real decision
points, so the set should cover:

| slot | shape | why it matters | grading |
|---|---|---|---|
| UC1 | Exploration-heavy change in a real repo (`repo:` in prompt.md): a feature or fix that needs reading several files/docs before editing | Only place the Haiku-subagent arm can differ; exercises `Explore`-style subagents | `check.sh`: tests/build pass, protected files untouched; `rubric.md`: minimality, correctness of reasoning |
| UC2 | PR review with pr-review-toolkit on a fixed diff | The team's daily workflow; subagents are frontmatter-pinned, so this shows what the presets change in practice | `rubric.md` only: findings vs a known issue list (recall), false positives, structure; optional `check.sh` for required sections |
| UC3 | Document/analysis task (e.g. a scoping-assessment section or an ADR from a brief) | Non-code output; tests whether the advisor adds anything when there is nothing to compile | `rubric.md` only, against a reference outline |

Per use case, set `max_turns` and `max_budget_usd` from one `--runs 1` pass (roughly 1.5x the
observed values), and `timeout_seconds` at 2x the observed wall time.

## Procedure

1. `python3 bench.py smoke` — auth, flags, which model each arm actually uses, judge works.
2. Write the three use cases; run each once on one arm (`--runs 1 -c light`) to calibrate caps.
3. `python3 bench.py run --runs 1` — one full pass; check per-run cost and `notes` column.
4. `python3 bench.py run --runs 3` (interleaved, default) — the real experiment. Add `--runs 2`
   more later if two arms are within noise on a use case.
5. `python3 bench.py summarize` — tables to `results/summary.md`; paste conclusions into the log.

Judge: `opus` at effort `high` (default). Opus is also a main model in three arms; acceptable, but if
Fable is available for judging, use `--judge-model fable` for the whole experiment and keep it fixed.

## Metrics and decision rule

Per (use case, arm), over repeats: pass rate (`check.sh`), median judge score (0–10), median wall
time, median `total_cost_usd`, median `advisor_cost_usd`, median turns, error count.

Proposed rule, per use case: rank arms by pass rate; among arms within one judge point of the best,
prefer the cheapest; report time as a secondary. An arm "wins" a use case if it is not worse on
pass rate, within one judge point, and at least 25% cheaper than the next candidate. Across use
cases, the recommendation is per task type, not one global preset.

## Budget

3 use cases x 5 arms x 3 repeats = 45 agent runs + 45 judge calls. From the one real run so far
(`light` at effort max, trivial fix: $0.46, 73 s), realistic tasks are expected at roughly $1–5 per
run on the Sonnet arms and $3–15 on the Opus arms at effort max, i.e. at most on the order of
$110–300 of estimated usage, plus judge calls (~$0.1–0.5 each). Only `normal-max` still runs at
max: Claude Code's own index puts xhigh at about half of max on Opus and medium at about a fifth
(Opus) to a tenth (Sonnet), so expect less. Calibrate with step 3 before step 4.
Costs are Claude Code's client-side estimates; on the Enterprise seat they map to plan usage.

## Threats to validity (and what the harness does about them)

- Run-to-run variance → ≥3 repeats, medians, min–max shown.
- Order effects (API latency drift, prompt cache warmth) → interleaved order, `cache_read_tokens` recorded.
- Advisor cost hidden → `modelUsage` includes the advisor; `advisor_cost_usd` column; runs where an advisor arm never lists the advisor are flagged in `notes`.
- Config drift → `--model` and `--effort` pinned in every arm; env overrides stripped; `claude_version` recorded per row.
- Uneven starting state → fresh worktree/copy per run, base commit recorded, diff saved.
- Grading bias → deterministic check first; judge model fixed, tool-less, sees rubric + diff + check verdict.
- Runaway runs → `max_turns`, `max_budget_usd`, harness timeout; capped runs count as failures.
- Effort confound → `normal-max` arm (`normal` at max).
- Subagent-model confound (Explore and Plan inherit the main model unless FORCE is on) → `light-force` arm.

## Findings so far

- 2026-10-09 — On `light` (Sonnet main, Opus advisor, then at effort max), one real run of `example-fix-tests`: total
  $0.455, Opus advisor $0.239 vs Sonnet main $0.216; advisor received 48k uncached input tokens;
  0 subagents spawned; check PASS, judge 9/10, 73 s. One data point; suggests the advisor is not a
  cheap add-on on small tasks.
- `--advisor` and `--max-turns` work in `-p` mode on 2.1.295 although absent from `--help`.
- On 2.1.295 a Haiku entry appeared in `modelUsage` on every run (internal helper calls), independent
  of subagents. None appeared in the 2026-10-15 calibration (2.1.300, parent-session variables
  removed), so there a Haiku entry would point at subagent work.
- 2026-10-15 — Calibration pass (step 3: `run --runs 1` on Claude Code 2.1.300, all five arms,
  `example-fix-tests`). All PASS with byte-identical fixes: `light` and `light-force` $0.15, 12 s,
  judge 10; `normal` $0.30, 17 s, judge 9; `high` $0.35, 25 s, judge 8; `normal-max` $0.39, 35 s,
  judge 8. The judge marked down only verbose final messages (Opus at xhigh and max added
  unrequested "extra checks"). 72–85% of each run's cost is writing the ~32–35k-token session
  context to the 1-hour cache at twice the input price, so on this task max cost 1.3x medium on
  Opus, not the catalog's 5.2x. No advisor was consulted and no subagent spawned, so `light` and
  `light-force` are indistinguishable here. The judge cost more than the five runs ($1.49 vs $1.35,
  ~50–90k tokens of context per Opus call); total $2.84.

## Log

- 2026-10-09 — Harness built and smoke-tested against Claude Code 2.1.295; four arms defined;
  launchers unified with configs via `install-launchers.sh`. Use cases: pending.
- 2026-10-15 — Added the `light-force` arm (`light` + `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1`). Read
  from the 2.1.300 binary: FORCE is on/off; with it every subagent ignores its frontmatter `model:`,
  `inherit` included, so only `light-force` runs Explore and Plan on Haiku. Forks and skills with
  `model: inherit` keep the main model; the Agent tool loses its `model` parameter.
- 2026-10-15 — Effort pinned per arm: `normal`, `light` and `light-force` at medium (Claude Code's
  default for Opus and Sonnet 5.5), `high` at xhigh. `post` had become identical to `light` and was
  replaced by `normal-max` (Opus at max, what the old `oclaude-normal`/`ccc` ran). Basis: the
  2.1.300 model catalog's `effort_cost_index` puts max at ~5.2x the default on Opus and ~10.2x on
  Sonnet.
- 2026-10-15 — Calibration pass (step 3) run; findings above. It was started from inside a Claude
  Code session with that session's variables removed by hand; `bench.py` now drops them itself
  (`SESSION_VARS`). Caps for the real use cases (UC1–UC3, still to be written) remain open.
