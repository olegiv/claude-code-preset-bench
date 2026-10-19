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
--no-session-persistence --no-chrome --strict-mcp-config --mcp-config bench-mcp.json --max-turns N
--max-budget-usd X`, so the only MCP server a run gets is Chrome DevTools (headless, throwaway profile).

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
| UC1 | Exploration-heavy change in a real repo (`lazygit-conflict-markers`: honor git's `conflict-marker-size` attribute in lazygit, upstream PR #5902, from its parent commit) | Only place the Haiku-subagent arm can differ; exercises `Explore`-style subagents | `check.sh`, in a container without network: 8 hidden integration tests (score = fraction passed) behind hard gates (`go.mod`, `go.sum`, `vendor/` untouched; the build) and soft gates (gofmt, vet, the workspace's unit tests, lazygit's other 512 integration tests); `rubric.md`: minimality, correctness of reasoning |
| UC2 | PR review with pr-review-toolkit on a fixed diff (`linkbox-review`: a planted pull request, about 550 lines, in a small Go service written for it) | The team's daily workflow; subagents are frontmatter-pinned, so this shows what the presets change in practice | `check.sh`: REVIEW.md's findings matched to 10 planted problems and 5 decoys, score = (found − decoys) / 10, PASS from 0.60; hard gates on REVIEW.md and an untouched pull request; soft gate when no pr-review-toolkit agent completed; `rubric.md` holds the answer key for explanations, severity and noise |
| UC3 | Document/analysis task (`linkbox-storage-adr`: ADR 0002 for linkbox, UC2's service, choosing how it stores links, from a brief, the team's notes and the code) | Non-code output; tests whether the advisor adds anything when there is nothing to compile, and whether an agent reads closely enough to reach the one decision that fits every constraint | `check.sh` gates only (one ADR with ADR 0001's sections, workspace untouched, no commit); `rubric.md`: the facts in the materials, the defensible decision (managed PostgreSQL), ten grounded points, bands |
| UC4 | Greenfield build with an algorithmic core and an htmx 4 UI (`meowdoku`: a 10×10 logic-puzzle web game in Go 1.27, htmx 4.0.0, Alpine.js 3.17.4) | Planning-heavy, with a real decision point (generating puzzles with exactly one solution), and it needs a library version newer than the models' training data | `check.sh`: 20 hidden Go acceptance tests (score = fraction passed) behind hard gates (go.mod and the two vendored scripts untouched; an offline build, which admits no third-party modules) and soft gates (gofmt, vet, own tests, `cmd/meowdoku` serving `-addr`, engine purity, `html/template`, `embed`), within a 750 s budget; `rubric.md` |

UC4 is written (`usecases/meowdoku/`). Its caps (200 turns, $40, 7200 s) come from the calibration
pass of 2026-10-17/18 (see the log). Grading it needs Go 1.27 or later, and perl, on the machine
running `bench.py`.

UC1 is written (`usecases/lazygit-conflict-markers/`). Its caps (100 turns, $25, 5400 s) come from
the calibration pass of 2026-10-18 (see the log). Running it needs Docker and, the first time,
network on the machine running `bench.py`, and Go 1.25 or later with git for the agents.

UC2 is written (`usecases/linkbox-review/`). Its caps (80 turns, $20, 3000 s) come from the
calibration pass of 2026-10-18 (see the log). Running it needs the pr-review-toolkit plugin
installed; runs get it and no other plugin.

UC3 is written (`usecases/linkbox-storage-adr/`). Its caps (60 turns, $15, 2400 s) are provisional
until a calibration pass. It needs nothing beyond git and python3 to grade.

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

4 use cases x 5 arms x 3 repeats = 60 agent runs + 60 judge calls; `run` takes every use case, `example-fix-tests`
included, unless they are named with `-u` (75 runs otherwise). From the one real run so far
(`light` at effort max, trivial fix: $0.46, 73 s), realistic tasks are expected at roughly $1–5 per
run on the Sonnet arms and $3–15 on the Opus arms at effort max, i.e. at most on the order of
$110–300 of estimated usage, plus judge calls (~$0.1–0.5 each). Only `normal-max` still runs at
max: Claude Code's own index puts xhigh at about half of max on Opus and medium at about a fifth
(Opus) to a tenth (Sonnet), so expect less. Calibrate with step 3 before step 4.
UC4's calibration (2026-10-18) puts its 15 runs alone at roughly $120–150 and 7–8 hours, most of
it `normal-max` and `high`.
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
- Timed-out runs lose their usage: Claude Code reports cost and turns only on exit → a `harness_timeout` row has neither; timeouts are set at 2x the slowest observed run so that runs finish.
- The owner's MCP servers and claude.ai connectors reached every run and the judge (163 tools in the UC4 calibration) → since 2026-10-18 runs get only the Chrome DevTools server in `bench-mcp.json` (headless, throwaway profile), the judge none, and Claude in Chrome is off for both. A machine with an administrator's `managed-mcp.json` cannot run that way (Claude Code exits at startup), so `run` and `smoke` refuse it.
- The owner's plugins reached every run and the judge, hooks included: security-guidance's Stop hook sends the git diff to an LLM review (on Opus, by default) and wakes the agent with its findings, outside the run's cost and turn counts. On UC1 and UC4 it reviewed the agent's own changes; on UC2 it reviewed the planted pull request, which hands any arm three of the ten problems → since 2026-10-18 runs get only the plugins their use case declares (`plugins:` in `prompt.md`), the judge and `smoke` none; Claude Code's built-in plugins stay. The UC1 and UC4 calibrations ran with it.
- Judge diff limit: 60 000 characters cut off the UC4 calibration diffs of `high` and `normal-max` (83 000–87 000) → raised to 120 000 on 2026-10-18; a longer diff is still cut off, and the rubric tells the judge not to penalise what it cannot see.
- UC2: ceiling → in calibration every arm found nine or ten of the ten planted problems, so the check barely separates the arms; the judge and the cost then decide.
- UC4: ceiling → every arm scored 1.00 in calibration, so the check may not separate the arms; the judge and the cost then decide.
- UC4, known limit (M13): a harness fault (no Go 1.27+, no perl) reaches the CSV and the judge as FAIL 0.00, like a hard-gate failure → only `results/raw/<run_id>.check.log` tells them apart.
- UC4, known limit (M16): a stalled handler's goroutine keeps running inside the hidden test binary (no process isolation) → watchdogs bound the wait, not the CPU it uses.
- UC4, known limit (M20): `cmd/meowdoku` is checked on a random `-addr`, never on its default `127.0.0.1:8080`, which may be in use on the grading machine.
- UC4, known limit: code that prints PASS lines and exits 0 before any test runs is still counted → closing it needs a results channel the code under test cannot write.
- UC4, known limit: variety is checked on two windows of 30 seeds (1–30, −30…−1) → a generator that collapses another window, such as 31–60, passes (self-test `generator-collapses-31-60`).
- UC4, known limit: determinism is checked on nine seeds (twice, and in a second process) → a generator that drifts on repeated calls for other seeds passes (self-test `generate-repeat-differs`).
- UC4, known limit: `TestHiddenConcurrent` creates four games at once under a 60 s watchdog each → a handler that holds its lock while generating, with a generator near 2 s (about 17 s under `-race`), fails although it meets the contract.
- UC4, accepted limit: the agent runs unsandboxed, so it can read or change `hidden/` and `check.sh` in the checkout, and read them in the public repository → nothing detects it; a sandbox would stop the changes, not the reading, since the prompt needs the web.
- UC1, accepted limit: the fix is public (lazygit PR #5902) and the agent has the web → the prompt asks it not to look the fix up; nothing detects it. The same holds for `hidden/`, as in UC4.
- UC1, known limit: the 8 acceptance tests sample the contract → a wrong fix they do not anticipate can still score 1.00; each case they cover came from a concrete wrong fix (most are self-test variants), and the rest is left to the judge.
- UC2: `code-reviewer` pins Opus, which is also `light`'s advisor model → `bench.py` counts that subagent's Opus usage as advisor cost and no longer flags an unused advisor; read `light`'s `per_model_cost_usd` and `models_used` with that in mind.
- UC2: pr-review-toolkit updates itself (version `ea10b213686c` at writing) → its agents, prompts and pinned models can change between runs; record the version with the results.
- UC2, accepted limit: the answer key is in the public repository, as UC1's and UC4's hidden tests are.
- UC1, known limit: agent code runs in the grading container, so it could print `go test -json` events for the hidden test names or write the grader's result files and fake a pass → as in UC4, closing it needs a results channel the code under test cannot write; links among the results are dropped, so it cannot make the grader print a host file.
- UC1: agents work on macOS, the grader runs Linux in Docker (as upstream's CI does) → a fix that works only on one of them would be graded on Linux; upstream's conflict tests pass on both.
- UC1: lazygit's integration tests run once each (`MaxAttempts: 1`) → a failed regression test is rerun once, and a run in which a test hangs (stopped by `go test -timeout`) is repeated once; acceptance failures are never rerun.

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
- 2026-10-15 — Added UC4 `meowdoku`; no model was called. The workspace holds `go.mod` and the
  vendored htmx 4.0.0 and Alpine.js 3.17.4, both checked against npm's sha512 integrity. A
  gitignored reference solution (`usecases/meowdoku/reference/`) passes `check.sh` at 1.00, and
  three reruns of the hidden tests against it all passed. `reference/selftest.sh` builds workspaces
  the way `bench.py` does. Results, the same under bash 5.3 and macOS `/bin/bash` 3.2: untouched
  workspace FAIL 0.00; reference PASS 1.00; `Check` ignoring diagonals FAIL 0.95 (TestHiddenCheck);
  unsorted or duplicated conflicts FAIL 0.95 (TestHiddenCheck); no uniqueness check FAIL 0.95
  (TestHiddenGenerateUnique); global rand FAIL 0.80 (TestHiddenGenerateDeterministic and three web
  tests); store without its mutex FAIL 0.95 (TestHiddenConcurrent, under `-race`); moves accepted
  after a win FAIL 0.95 (TestHiddenWin); one byte changed in `htmx.min.js` FAIL 0.00; a third-party
  import FAIL 0.00; a file not gofmt'ed FAIL 1.00; own tests removed FAIL 1.00. The reference's
  `Generate` over seeds 1–200 on the development Mac (darwin/arm64), without `-race`: p50 0.8 ms,
  p95 3.6–4.2 ms, max 6–13 ms (three runs); it never fell back to its unique-by-construction
  layout in 20 000 seeds. `check.sh` on the reference takes about 8 s warm and 18 s with an empty
  Go build cache. The reference's diff is about 40 000 characters, under the judge's 60 000 limit.
  Caps stay provisional until calibrated.
- 2026-10-16 — UC4 reviewed twice (Claude Code, Codex). The findings merge into 29 items and 4
  nits; every blocking and should-fix item was reproduced with a self-test variant or a probe on
  the unchanged grader before anything was changed. Items: M1 go test's own vet run failed every
  hidden engine test; M2 the variety test has no contract clause; M3 `-race` made a compliant
  generator miss the 10 s request watchdog; M4 cells identified by `data-row` anywhere; M5 printed
  PASS lines counted, exit codes ignored; M6 `won` matched as a substring; M7 script URLs ignored
  `<base>` and accepted a CDN host; M8 `VERSIONS.txt` gated, though the prompt protects two files;
  M9 `data-region` tied to the `Regions` numbering; M10 `GOFLAGS` from the go env file; M11 the
  2 s limit doubled as the call deadline; M12 concurrent game creation untested; M13 a harness
  fault reaches the judge as a hard-gate 0.00; M14 no correct-alternative variants; M15 bash 3.2
  left the temp dir when logging; M16 concurrent workers could hit the package timeout; M17 no
  total time bound (an `init` hang outlived `-timeout`); M18 the shared `Generate` cache; M19
  watchdogs posing as performance limits; M20 a missing `cmd/` passed; M21 an empty `go.sum`
  passed; M22 405 coverage and HEAD; M23 an empty start page passed; M24 JSON types unchecked;
  M25 engine gaps (cross-process determinism, limits −1 and above the count, `.Seed`); M26 web
  gaps (constant random seed, negative seeds, non-row conflicts, the full page after moves, `won`
  on other responses, lost updates); M27 clauses without checks (purity, `html/template`, `embed`,
  double quotes); M28 the board-unchanged-after-400 assertion has no contract clause; M29 the
  rubric demands a solver proof.
  Fixed: M1, M3–M7, M9–M12, M14–M17, M19, M20, M22 (except HEAD), M23–M27, and nit N3. The grader
  now runs `go test -json -vet=off` and counts results with a small tally tool (a failure beats a
  printed pass; results keyed by package; non-zero exits fail the check), sets `GOENV=off`, owns its
  temp dir at top level, runs every step in a process group inside an 840 s budget (web tests
  `-timeout 450s`, up from 180 s, because a compliant generator is about ten times slower under
  `-race`), and adds soft gates for `cmd/meowdoku`, engine imports, `html/template` and `embed`.
  The hidden tests got 20 s and 60 s watchdogs (the 2 s limit is asserted only by the speed test),
  a markup tokenizer that honours nesting, comments and raw text, `<base>`-aware same-origin script
  checks, parsed `HX-Trigger`, partition comparison for `data-region`, JSON type checks, and the
  coverage listed above; the hidden test count stays 20. Waiting for the owner: M2 (both reviews
  accept dropping the test), M8, M13 (needs `bench.py` or the rubric), M21, M22's HEAD part (both
  accept exempting HEAD in the prompt), M28, nits N2 and N4 (rubric and prompt text). Not
  reproduced: M18 (a generator that changes its answer on repeated calls passes with and without
  the cache; the reviews also disagree on the fix), M29 (judge only), and M26's float storage
  (already caught). Found while fixing: code that prints PASS lines and exits 0 before any test
  runs is still counted, and determinism is sampled on nine seeds only.
  Self-test: 60 variants. The default group (52) matched under bash 5.3 and `/bin/bash` 3.2; the
  pending group (5) still reproduces the open items; the slow group (3) matched: a 1.7 s generator
  passes in 539 s, a deadlocked store fails without a package timeout, a hanging `init` stops at
  the budget (843 s with the self-test's overhead). Reference: three reruns at 20/20; `check.sh`
  takes 11 s warm and 22 s with an empty Go build cache.
- 2026-10-16 — Decisions on the open UC4 items, applied. Prompt: no longer forbids a `go.sum`;
  states that any 30 consecutive seeds yield at least 27 distinct region layouts (M2, the
  variety test stays); accepts HEAD wherever GET is (M22); says a rejected request (any 4xx)
  leaves the game unchanged (M28, the assertion stays). Grader: `VERSIONS.txt` left the hard gate
  (M8) and the `go.sum` gate is gone (M21; an unchanged `go.mod` and `GOPROXY=off` already refuse
  third-party modules); the 405 checks use DELETE, PUT, GET on POST-only routes and POST on
  GET-only routes, never HEAD. Rubric: the 7–9 band also covers a check FAIL only on a soft gate
  (gofmt, vet, own tests) with score 1.00. No `bench.py` change (M13), the cache stays (M18), and
  M13, M16, M20, the faked-pass case and nine-seed determinism are recorded as known limits under
  Threats to validity. Self-test after the decisions: the default group, now 56 variants, matched
  under bash 5.3 and `/bin/bash` 3.2; the `limits` group (the nine-seed determinism case) still
  reproduces its limit; the slow group (3) matched (a 1.7 s generator passes in 533 s); three
  reference reruns passed 20/20. Full table in `usecases/meowdoku/README.md`.
- 2026-10-16 — Before the commit. Six correct-but-different builds were added for the structural
  gates (M20, M27): an engine using `fmt`, templates and their `embed` in a `web` subpackage, the
  scripts embedded by a package in `web/static`, `text/template` next to `html/template`, and a
  command that starts slowly and spawns a helper process; all pass. A seventh, the scripts embedded
  in `cmd/meowdoku` and handed to `web`, cannot pass under the contract (`NewHandler()` alone must
  serve them; `//go:embed` cannot reach `web/static` from `cmd`), so it stays as evidence with an
  expected FAIL. The engine-imports gate now reads engine's direct imports only (`go list -f
  '{{join .Imports " "}}'`; denylist `os os/exec os/signal net net/http io/ioutil log log/slog
  syscall crypto/rand`); the `-addr` gate takes a port the OS reports free and accepts an answer
  only while its server is running, then kills the server's process group; the budget dropped
  from 840 s to 750 s, and `check.sh` prints its total time (a hanging `init` now ends at 753 s).
  The rubric's 7–9 band now reads "a soft gate (any check that fails without lowering the score)".
  A full run under `/bin/bash` showed `concurrent-create-unlocked` passing once: the creations had
  outlasted the readers, so no read overlapped the unlocked insert. The readers now keep reading
  until every concurrent creation has finished.
- 2026-10-16 — PR 2 review (Codex, four P1). `check.sh` forced `CGO_ENABLED=1`, so on a Linux
  host without a C toolchain the hard-gate build failed in `runtime/cgo` (`net` uses cgo there)
  before the race probe could turn `-race` off; reproduced with a Linux cross-build. The probe now
  runs right after the harness check and sets `CGO_ENABLED=0` when it fails. Alpine.js's MIT
  notice now ships next to the vendored file (its npm tarball has none; htmx is 0BSD). The claim
  that the hidden tests never reach the agent was wrong: an unsandboxed arm can read or change
  them, and read them on GitHub; recorded as an accepted limit under Threats to validity. Not
  changed: committing the grader self-test, since the reference solution stays out of this
  public repository by design. The local review added two items. The script and start-form
  checks compared a resolved URL with the `<base href>` origin, so a base on a CDN made CDN
  scripts count as local; they now compare with the page's own origin. A `vendor/` directory
  could not bring in third-party code on go1.27.2 (six `modules.txt` layouts tried, all refused
  while `go.mod` is unchanged), but `GOFLAGS=-mod=readonly` now ignores it anyway. Its second
  pass found three more, fixed on the owner's override without a further review: the start-page
  check now accepts `data-hx-post` and `data-hx-action` as htmx does; the state parser rejects
  `null` inside `regions` and `conflicts` (`json.Unmarshal` made it 0); and every bounded step
  now kills what is left in its process group when it exits normally, not only on timeout.
- 2026-10-16 — PR 2 review, second pass (Codex, one P1, two P2). Nothing capped what the
  candidate's code writes into the grader's logs, so a test that floods stdout could fill the
  disk within its time cap. Every step, the `-addr` server included, now runs under `ulimit -f`
  at 128 MB (Go's own build files stay under about 15 MB; at 1 MB the linker fails), and a log
  that reaches the cap fails the check. Go ignores `SIGXFSZ`, so a flooding program gets write
  errors and runs on to its time cap without growing the file. Without curl the `-addr` gate
  only warned, so a run could pass unchecked; it now fails. The markup tokenizer took
  `<script … />` as closed, while browsers ignore the `/` and read on as script text; `script`,
  `style`, `textarea` and `title` now ignore it too. The local review added two items: the
  redirect check accepted a `Location` on another server as long as its path was `/games/{id}`
  (it now also requires this server's scheme and host), and four serialized creations by a
  lock-holding handler with a near-limit generator can outlast the 60 s watchdog (recorded as a
  known limit).
- 2026-10-17 — PR 2, round 3 under the owner's override (both fix pushes had been used).
  PRRT_kwDOVKtXp86tRgSD: the `-addr` probe now runs `curl --noproxy '*'`, so proxy variables on
  the grading host cannot send the loopback check elsewhere. PRRT_kwDOVKtXp86tRgR2:
  `TestHiddenGenerateVaried` also checks seeds −30…−1, which catches a generator that clamps or
  folds negative seeds; the prompt's "any 30 consecutive seeds" can only be sampled, and the added
  calls keep the engine package inside its 180 s timeout. PRRT_kwDOVKtXp86tRgR8 (`seq` missing on
  macOS) was rejected: `/usr/bin/seq` ships with macOS.
- 2026-10-18 — UC4 calibration (steps 2–3): one run per arm, no judge, Claude Code 2.1.302. Every
  arm built a game that passed all gates and 20/20 hidden tests: `light` 557 s, $0.87, 32 turns;
  `light-force` 661 s, $0.82, 27 turns; `normal` 704 s, $3.13, 69 turns; `high` 2527 s, $13.88
  (Fable $4.41), 127 turns. `normal-max` hit the 3600 s timeout; the code it left passed at 1.00,
  but a killed run reports no usage, so its cost and turns are unknown (under $30, since the budget
  cap never stopped it). The check does not separate the arms here, and on cost `light` is 4–16x
  cheaper. Sonnet never called its Opus advisor (0 of 4 advisor-arm runs so far, example-fix-tests
  included); a direct check (the `light` config told to call the advisor, $0.32) showed that the
  tool works, so the model chooses not to. Opus at xhigh did call Fable. No completed run spawned
  a subagent (`normal-max`'s counts were lost with its usage), so FORCE had nothing to act on in
  `light-force`. Every run that wrote a final
  message says it tested the game in Chrome, through the owner's MCP servers. Diffs were
  36 000–52 000 characters, but 83 000 (`high`) and 87 000 (`normal-max`), past the judge's
  60 000. Caps set to 200 turns (`high` used 127; 1.5x is 190), $40 (`high` $13.88; `normal-max`
  under $30 after an hour, unfinished) and 7200 s (`high` 2527 s; `normal-max` over 3600 s); the
  `normal-max` values are estimates, to be raised if its first real run hits one. The prompt's
  "start with the migration guide" led to a 404 or to the htmx 2 guide
  (`four.htmx.org/migration-guide` redirects to `v2.htmx.org`); it now names
  `https://four.htmx.org/docs/whats-new-in-htmx-4`. The `light` run had the old line (its agent hit
  the 404), so it is not a controlled comparison with the other four; with `light-force`, which had
  the same prompt as the Opus arms ($0.82), the Sonnet arms are still 4–17x cheaper.
- 2026-10-18 — Runs and the judge isolated from the owner's tools, before any real run. A
  1-turn Haiku session with the old flags started with 31 built-in tools and 163 MCP tools: two
  Chrome DevTools servers (the owner's and the plugin's) and the claude.ai connectors (Atlassian,
  Figma, Stripe, Claude Docs); other plugin servers needed auth or failed. The judge had no
  built-in tools (`--tools ""`) but the same 163 MCP tools. Claude in Chrome does not load in `-p`
  mode, so the calibration agents tested in Chrome DevTools' own profile, not the owner's browser.
  Now runs get `--no-chrome --strict-mcp-config --mcp-config bench-mcp.json`, which holds
  `chrome-devtools-mcp@1.11.0 --isolated --headless` (latest stable, no runtime dependencies,
  `npm audit` clean), and the judge `--no-chrome --strict-mcp-config` alone. The same probes then
  showed a run with 28 built-in tools (only the three MCP resource tools gone; WebFetch and
  WebSearch kept) and the 32 Chrome DevTools tools, and the judge with no tools at all, so the 163
  tool definitions no longer sit in every judge call's context (the two judge probes cost $0.41 and
  $0.04, single calls whose cost also depends on cache state). Headless DevTools sessions, built
  by `build_agent_cmd` in its real argument order, opened example.com and read its title, and a
  1-turn cap still stopped the run (`error_max_turns`). The judge's diff limit went from 60 000 to 120 000 characters, so every UC4 calibration
  diff (up to 87 000) would now be seen whole; the rubric says so. The configs are unchanged, so
  the `oclaude-*` launchers keep the owner's MCP servers.
- 2026-10-18 — PR 4 review (Codex, one P1, one P2). Claude Code's managed-MCP documentation says
  it exits at startup when given `--strict-mcp-config` on a workstation with an administrator's
  `managed-mcp.json`, so on such a machine every run and judge call would have failed as `no_json`;
  `run` (except `--dry-run`) and `smoke` now refuse it, naming the file. `npx -y` fetched the
  DevTools package inside the first timed run when npm's cache lacked it; `run` now starts each
  server once with `--help` before scheduling. With a throwaway npm cache that took 0.9 s and left
  the package cached; with npm forced offline it stopped the run before any model call.
- 2026-10-18 — UC1 written: `lazygit-conflict-markers`; no model was called. A read-only search of
  recent merged PRs in permissively licensed Go projects gave four candidates (lazygit #5902, #6099
  and #5924, kin-openapi #1290); the owner chose lazygit #5902, which makes lazygit honor the
  `conflict-marker-size` attribute (8 files in 6 packages, merged 2026-08-08). In Docker
  (`golang:1.27.2`, no network), all 513 integration tests and the unit tests pass at the merge
  commit (18 s and 12 s); at the parent with the PR's two integration tests added, exactly those two
  fail, for the reasons in the issue (the merge-conflict editor does not open; the file is staged),
  in 3 of 3 runs. Three acceptance tests of ours (seven-character marker lines are content, a length
  of 12 set by a pattern, all hunks resolved and then the continue prompt) and one regression test
  (other files keep the default) behaved the same way in 5 of 5 runs. Upstream's conflict tests also
  pass on macOS (16 of 16, git 2.56), where agents work. The grader runs in Docker, as upstream's CI
  runs these tests on Linux and it keeps agent code off the host while grading; it replaces the
  agent's integration-test tree with upstream's and the hidden tests, and treats `go.mod`, `go.sum`
  and `vendor/` as a hard gate. `setup.sh` removes upstream's `AGENTS.md` and `CLAUDE.md`, which
  prescribe commits, `just` and a gopls MCP server, and commits that. The first full self-test had
  one grading container stall until its 600 s limit with no results; reruns did not repeat it. A run
  in which a test hangs now ends at `go test -timeout`, inside the container's limit, keeps the
  results so far and is repeated once. Two local Codex reviews (126 s and 117 s) then found five
  defects, each shown failing before its fix: a unit test in the workspace could swap a result log
  for a link to a host file, which the grader printed (P1; links among the results are now dropped);
  a `pkg/integration` linked outside the workspace would have let the grader delete through the link
  (P1; now a hard gate); the timeout rerun replaced first-run results, so a failed acceptance test
  could pass on the rerun (it now only fills in tests without a result); the unit tests had no
  timeout, so a hanging one cost every result (360 s, then a harness fault; now 60 s and a soft
  gate); and `setup.sh` did not refresh the cache when the pins change. The self-test's docker check
  now uses a failing stub instead of a `PATH` without `/usr/local/bin`. Self-test: all 18 variants
  gave the expected verdict under bash 5.3 and `/bin/bash` 3.2 (8 minutes each), among them the
  reference at 1.00, the untouched workspace at 0.00, a hard-coded length at 0.80, the auto-staging
  check left on the default length at 0.60, and the length applied to every file at 1.00 with 32
  regressions. Caps are provisional.
- 2026-10-18 — PR 5 review (Codex, two P1), both about gaps in UC1's hidden tests. Every scored test
  had a single conflicted file, so a fix that gave all conflicted files one file's marker length
  passed; and every length came from the root `.gitattributes`, so a fix that parsed only that file
  passed. Two acceptance tests of ours close them: `mixed_sizes` (one merge with a default-length
  file and a length-12 file) and `other_attribute_sources` (lengths 20 and 16 from a subdirectory's
  `.gitattributes` and from `.git/info/attributes`). Both fail at the base (a premature continue
  prompt; the editor does not open) and pass at the reference, in 5 of 5 runs each way. The score now
  counts 7 acceptance tests; a new self-test variant that applies the first file's length to every
  file scores 0.71 (the five earlier tests all pass on it), and the other partial variants drop to
  0.43–0.86.
  Self-test: all 19 variants gave the expected verdict under bash 5.3 and `/bin/bash` 3.2.
- 2026-10-18 — PR 5, second review (Codex, three P1), again wrong fixes that the hidden tests let
  through: `git check-attr` output parsed on whitespace (a path with a space loses its length), lengths
  looked up only during a merge (every conflict came from `git merge`), and markers matched as "at
  least N characters" (no longer marker-like block). `mixed_sizes` now names its length-12 file
  `my notes.md`, `short_markers_are_content` also carries a 33-character block in a length-32 file,
  and a new acceptance test, `rebase`, resolves a conflict of a rebase started in lazygit (the base
  stages the still-conflicted file). All 8 acceptance tests fail at the base and pass at the
  reference, 5 of 5 runs each way. Self-test variants for the three wrong fixes each fail exactly
  their test and score 0.88 (they scored 1.00 before). This was the PR's last fix push; the hidden
  tests are recorded as a sample of the contract (threats above), so further reports of that kind
  will be triaged. Self-test: all 22 variants gave the expected verdict under bash 5.3 and
  `/bin/bash` 3.2 (10 minutes each).
- 2026-10-18 — UC1 calibration: one run per arm, no judge, Claude Code 2.1.302. Every arm's change
  passed all gates and 8/8 acceptance tests: `light` 281 s, $0.68, 24 turns; `light-force` 474 s,
  $0.76, 28 turns; `normal` 328 s, $1.60, 32 turns; `high` 877 s, $7.03 (Fable $3.42), 64 turns;
  `normal-max` 2640 s, $13.72. As on UC4, the check does not separate the arms, and on cost
  `light` is 2–20x cheaper. The `high` row reads FAIL 0.00: Docker Desktop's engine stopped while it
  was graded (`FAIL: harness: the Docker daemon is not running`), and the next three runs failed in
  `setup.sh` before any agent started, as intended. Re-graded from its saved diff in a fresh
  workspace once Docker was back, `high` passes at 1.00; the three runs were repeated (those
  `harness_error` rows stay, as the log is append-only). Sonnet again never called its Opus
  advisor (0 of 5 so far). `normal-max` is the first arm to spawn a subagent: a background
  `pr-review-toolkit:code-reviewer` (runs still get the owner's plugins; only MCP servers and
  Claude in Chrome are isolated), which the `-p` session killed when it ended, with the final
  message still "waiting for the independent review"; its row reports 2 turns, so turn counts are
  unreliable when a background subagent is involved. Caps set to 100 turns (`high` used 64; 1.5x
  is 96), $25 (`normal-max` $13.72; 1.5x is $20.6) and 5400 s (`normal-max` 2640 s; 2x is 5280).
  The calibration cost $23.79.
- 2026-10-18 — UC2 written: `linkbox-review`; no model was called. linkbox is a Go service of about
  800 lines written for it; the pull request (+547/−16 in 9 files) is stored as `feature.patch`,
  so the repository's code scanning does not flag its planted problems, and `setup.sh` applies it
  uncommitted with new files intent-to-add, which is how `/pr-review-toolkit:review-pr` finds a
  change. Ten planted problems span the toolkit's areas (SSRF and path traversal, a cache race, a
  body leak, a weak test, an authorization hole, a 0-based page, a dropped error, a wrong comment, a
  store returning internal pointers); five decoys look like a goroutine leak, byte truncation, path
  traversal, a double close and ignored write errors, and are correct. The base and the pull request
  build, pass `go vet` and their tests with `-race`. The prompt names the review aspects because
  `review-pr` otherwise ends with `code-simplifier`, which edits code. Findings count at the first
  line they cite and only the first 25 are read; the self-test's 18 reviews give the perfect review
  1.00, seven of ten 0.70, evenly spaced spam 0.10 and whole-file ranges 0.00. The README's layout
  block had meowdoku's last two lines under lazygit since PR 5; they are back in place.
- 2026-10-18 — PR 7 review (Codex, one P1): a run that committed REVIEW.md, or the pull request
  with it, still passed every gate, although the prompt forbids commits and the judge's diff
  (working tree against `HEAD`) would then miss the work. `setup.sh` now records the starting
  commit in `.git/ccbench-base`, and `check.sh` hard-fails a run whose `HEAD` moved; a new
  self-test variant, `committed`, scored 1.00 PASS before the fix and is a hard 0.00 after it.
- 2026-10-18 — UC2 calibration, first run (`light`, no judge, Claude Code 2.1.302): PASS 0.80 in
  124 s, $1.71, 15 turns, all five toolkit agents completed. It found all ten planted problems, but
  two valid findings about other problems landed on decoy lines: CSV formula injection at
  `WriteCSV`'s `Write` call (the `csv-sticky-error` decoy) and `Cleanup` stopping at its first
  `os.Remove` error (`cleanup-remove`). Its last message answered findings from security-guidance's
  Stop hook, which had reviewed the planted pull request: the owner's plugins reach runs. Two
  changes before the other arms: `bench.py` gives runs only the plugins their use case declares
  (UC2: pr-review-toolkit) and the judge and `smoke` none, turning the others off with
  `--settings`; probes showed that this unloads installed and claude.ai-synced plugins alike and
  leaves only Claude Code's four built-in ones. In the pull request, `WriteCSV` now escapes
  formula-leading cells and `Cleanup` carries on past a file it cannot remove, so the decoy lines
  invite nothing else (+623/−16 by `git apply --numstat`; only `cleanup-remove`'s windows moved).
  The calibration restarts on all five arms; this run's row describes the earlier version.
- 2026-10-18 — UC2 calibration: one run per arm, no judge, Claude Code 2.1.302, the first runs
  with only pr-review-toolkit loaded (no run mentions a hook). Every arm passed, and in every run
  all five toolkit agents completed: `high` 0.90 (1482 s, $12.37), `normal-max` 0.90 (1067 s,
  $12.30), `normal` 0.90 (1266 s, $2.58), `light` 0.90 (367 s, $1.35) and `light-force` 1.00
  (293 s, $0.48). Every arm found all ten planted problems except `light`, which missed the
  `?owner=` bypass. The three Opus arms each lost 0.10 on the `ticker-goroutine` decoy with the
  same valid finding, cited at the ticker line: the first export cleanup runs an hour after
  startup. The cleanup goroutine now also sweeps at startup (the decoy's windows moved to 93–94
  and 97–98; the pull request is +627/−16), which leaves the caps as they are. Without those hits
  the check reads 1.00 for every arm but `light`: a ceiling, as in UC1 and UC4, so the judge and
  the cost decide. `light-force`, with every subagent on Haiku, matched the Opus arms' findings
  for $0.48 against `high`'s $12.37. Turn counts read 1–8 with background subagents and are
  unreliable. Caps: 80 turns kept, $20 kept (1.5x $12.37 is $18.6), 3000 s (2x 1482 s). The
  calibration cost $29.09.
- 2026-10-19 — UC3 written: `linkbox-storage-adr`; no model was called. The agent writes ADR 0002
  for linkbox (UC2's base service, copied) from a brief, seven short notes in `docs/` and the
  code. The notes are written so that one store fits every constraint: the platform requires at
  least two replicas without session affinity and offers no persistent volumes, which rules out
  SQLite and bbolt; Redis is a cache tier without backups; object storage has no conditional
  writes and 20 to 80 ms per request against a 100 ms p99 target; teams may not run their own
  databases. That leaves the managed PostgreSQL. Two traps: the brief leans towards SQLite and
  ADR 0001 still says linkbox runs as a single instance. Since every strong arm should reach
  PostgreSQL, the rubric puts its points on grounded consequences (the tickets explained by
  replica divergence, the `l<n>` counter, the store's API, a pool sized for four instances,
  retention under the 30-day erasure rule, migrations under concurrent starts) and caps any
  other store at 3. `check.sh` only gates, with UC2's HEAD-moved gate, since a commit would
  hide the whole ADR from the judge; `selftest.sh` checks that each fact the rubric cites is in
  the materials and runs the gates on 18 workspace states. Caps are provisional.
