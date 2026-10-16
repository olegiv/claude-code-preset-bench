# meowdoku — a greenfield Go + htmx 4 build

UC4 in [`PLAN.md`](../../PLAN.md). Every benchmark arm gets the same prompt and an almost empty
workspace, and builds **Meowdoku**: a single-player logic puzzle in which ten cats go on a 10×10
board split into ten regions, one per row, column and region, with no two cats touching. The app is
Go 1.27 (standard library only), server-rendered with `html/template`, and uses the vendored htmx
4.0.0 and Alpine.js 3.17.4.

Why it is in the experiment:

- **Planning-heavy.** An engine package, a web package, a command, templates and tests, all from
  scratch, against an exact contract.
- **A real decision point.** Every generated puzzle must have exactly one solution, every seed must
  terminate, and each `Generate` call must finish in under 2 s. How the agent proves uniqueness is
  the core design choice.
- **A library newer than the models' training data.** htmx 4 changes defaults that models know from
  htmx 2 (error responses are swapped, inheritance is explicit, events are renamed, `HX-Trigger`
  fires after the swap). The prompt points the agent at the htmx 4 docs instead of memory.

It is graded twice: `check.sh` runs 20 hidden Go acceptance tests behind a few gates and reports the
fraction passed as the score, and the judge scores the build from 0 to 10 with `rubric.md`.

Nothing in this folder except `workspace/` reaches an agent: `bench.py` copies only `workspace/`
into each run's temporary directory and passes only the body of `prompt.md`. This README, `hidden/`
and `check.sh` stay here.

## Files

| Path | Committed | What it is |
|---|---|---|
| `prompt.md` | yes | Frontmatter (caps) and the prompt, including the contract the hidden tests exercise |
| `workspace/go.mod` | yes | Exactly `module meowdoku`, a blank line, `go 1.27` |
| `workspace/.gitignore` | yes | `/meowdoku`, `/bin/`, `*.test`, `*.out`, so built binaries stay out of the run's diff |
| `workspace/web/static/htmx.min.js` | yes | htmx.org 4.0.0, `package/dist/htmx.min.js` from the npm tarball |
| `workspace/web/static/alpine.min.js` | yes | alpinejs 3.17.4, `package/dist/cdn.min.js` from the npm tarball |
| `workspace/web/static/VERSIONS.txt` | yes | Package, version, tarball URL and SHA-256 of both files (informational; not gated) |
| `hidden/engine/zz_hidden_*_test.go` | yes | 9 acceptance tests, `package engine_test`, plus the test oracle |
| `hidden/web/zz_hidden_*_test.go` | yes | 11 acceptance tests, `package web_test`, plus a copy of the oracle |
| `check.sh` | yes | The deterministic grader |
| `rubric.md` | yes | The judge's rubric |
| `README.md` | yes | This document |
| `reference/solution/` | **no** | A complete reference build, overlaid on `workspace/` to test the grader |
| `reference/selftest.sh` | **no** | The grader self-test |

`reference/` is ignored by the root `.gitignore` (`usecases/*/reference/`), so it exists only on the
machine where it was written. A fresh clone has no reference solution and no self-test; the
sections below that use them say so.

## Requirements

- **Go 1.27 or later** on the machine that runs `bench.py`, for grading. `check.sh` sets
  `GOTOOLCHAIN=local`, so Go never downloads a toolchain, and refuses an older Go as a harness
  problem (exit 2, score 0.00).
- **No network for grading.** `check.sh` sets `GOPROXY=off`, so a third-party import fails the build.
- **perl**, which `check.sh` uses to run each step in its own process group under a time limit,
  and **curl**, for the `-addr` check (without curl that check is skipped with a warning).
- **A C toolchain** (Xcode Command Line Tools on macOS) for the race detector. Without one,
  `check.sh` runs the web tests without `-race`, prints `WARN: no -race`, and can no longer catch an
  unguarded game store.
- **bash** 3.2 or later (macOS `/bin/bash` works), git, and Python 3.8+ for `bench.py`.
  `selftest.sh` also uses `perl`, `dd`, `cmp`, `awk` and `gofmt`.

## Running it

From the repository root.

```bash
python3 bench.py list                                          # meowdoku: turns, budget, timeout, check.sh, rubric.md
python3 bench.py run --dry-run --runs 1 --no-judge -u meowdoku # one command per preset; no model calls
```

### Calibrate the caps first

The caps in `prompt.md` (150 turns, $30, 3600 s) are provisional. Calibrate them with one run on the
`light` preset:

```bash
python3 bench.py run --runs 1 -c light -u meowdoku
```

Then, as `PLAN.md` prescribes, set `max_turns` and `max_budget_usd` to about 1.5× the observed turns
and cost, and `timeout_seconds` to 2× the observed wall time. They are the frontmatter keys at the
top of `prompt.md`; record the new values and the run they came from in the log at the end of
`PLAN.md`.

### The experiment

```bash
python3 bench.py run --runs 3 -u meowdoku    # every preset x 3, interleaved
python3 bench.py summarize                   # tables on stdout and in results/summary.md
```

Useful options: `-c high light` (only some presets), `--no-judge`, `--judge-model fable`,
`--keep-workspaces` (keep each run's workspace on disk to inspect or re-grade it).

**Cost.** Each run can spend up to its `max_budget_usd`: with the provisional caps, one repeat over
the five presets can reach about $150 of estimated usage, plus one judge call per run. Calibrate
before scaling up.

## Reading the results

Each run adds a row to `results/runs.csv`. The meowdoku-specific columns:

| Column | Meaning |
|---|---|
| `check_pass` | 1 if `check.sh` exited 0: every gate passed and all 20 hidden tests passed |
| `check_score` | The fraction of hidden tests passed, two decimals; 0.00 when a hard gate failed |
| `judge_score` | The judge's 0–10 score from `rubric.md` |

`check.sh` prints much more than the score, but only its last line reaches the CSV and the judge.
The full output goes to the gitignored `results/raw/<run_id>.check.log`, next to the run's other
raw files (`.json`, `.result.md`, `.diff.patch`, `.stderr.txt`, `.judge.json`). Read the log to see
which gate or hidden test failed. A typical log:

```
ok: harness: go1.27.2
ok: go.mod and the vendored scripts unchanged
ok: go build (1s)
ok: gofmt
ok: go vet (0s)
ok: own tests in engine/ and web/
ok: go test ./... (1s)
ok: cmd/meowdoku builds and serves GET / on -addr
ok: engine imports no I/O packages
ok: web uses html/template
ok: web embeds the vendored scripts
hidden engine tests ran (1s, exit 0)
hidden web tests ran (6s, -race, exit 1)
    ... the output of the hidden tests ...
hidden 19/20; failed: TestHiddenWin
time: 14 s (budget 750 s)
FAIL: 0 gate(s) failed, hidden 19/20
0.95
```

A failed gate prints a `FAIL: ...` line, followed by the tail of the tool's output where there is
one; a step stopped by its time limit says `(stopped: time limit)`. A hard-gate failure stops
there with `FAIL: hard gate` and `0.00`. A line starting `FAIL: harness:` means the grading
machine, not the agent, is at fault (no Go, Go older than 1.27, no perl). The CSV and the judge see
that as a failed check with 0.00 too, so look for it in the log before trusting a 0.00.

The judge sees the rubric, the prompt, the agent's final message (up to 12 000 characters), the
workspace diff (up to 60 000 characters) and the check verdict as `PASS (score 1.00): 1.00`. The
reference solution's diff is about 40 000 characters, so a complete build is normally seen whole.

## How `check.sh` grades

`bench.py` runs `bash check.sh` with the workspace as the working directory and a 900 s timeout.
Every `go` command runs with `GOTOOLCHAIN=local GOPROXY=off GOWORK=off GOFLAGS= GOENV=off
CGO_ENABLED=1`; `GOENV=off` ignores a go env file the agent may have written with `go env -w`.

The whole check has a budget of 750 s. Each step runs in its own process group under a cap, or
under what is left of the budget if that is less, and the whole group is killed when time runs
out, so nothing outlives the check. The worst case ends a few seconds after the budget (the
stopped step's grace period, then the summary), under 780 s; `check.sh` prints its total time on
the line before its verdict. Caps: `go build` 300 s, `gofmt` 120 s, `go vet` 300 s, the
agent's tests 300 s (with `-timeout 240s`), building `cmd/meowdoku` 180 s, each `go list` 120 s,
the hidden engine tests 240 s (`-timeout 180s`) and the hidden web tests 480 s (`-timeout 450s`).

1. **Harness check.** `go` and `perl` must be on `PATH` and `go env GOVERSION` must be go1.27 or
   later. Otherwise: `FAIL: harness: ...`, `0.00`, exit 2.
2. **Hard gates.** Any failure scores 0.00 and exits 1:
   - `go.mod`, `web/static/htmx.min.js` and `web/static/alpine.min.js` are byte-identical to
     `workspace/` here (`cmp -s`);
   - `go build ./...` succeeds offline. With `go.mod` unchanged and `GOPROXY=off`, that also refuses
     third-party modules, so a `go.sum` is not checked.
3. **Soft gates.** A failure fails the check but keeps the score:
   - `gofmt -l .` prints nothing;
   - `go vet ./...` passes;
   - `engine/` and `web/` each contain at least one `func Test` in a `_test.go` file;
   - the agent's own `go test -count=1 -timeout 240s ./...` passes;
   - `cmd/meowdoku` builds and answers `GET /` on the address given by `-addr`. The port is one the
     OS reports free; the check waits up to about 15 s for an answer while the server process is
     still running (so no other process on that port can pass for it), then kills the server's
     whole process group. The default `127.0.0.1:8080` is not tried, since it may be taken on the
     grading machine;
   - `engine`'s direct imports (`go list -f '{{join .Imports " "}}' ./engine`) include none of
     `os`, `os/exec`, `os/signal`, `net`, `net/http`, `io/ioutil`, `log`, `log/slog`, `syscall`,
     `crypto/rand`: a best-effort denylist, not a proof of purity (`fmt`, which reaches `os`
     indirectly, is fine);
   - `web`, or a package it imports, uses `html/template` (using `text/template` as well is fine);
   - `web`, or a package of the module it imports, embeds `htmx.min.js` and `alpine.min.js` with
     `embed`.
4. **Hidden tests.** The workspace is copied, without `.git`, into a temporary directory that is
   removed on exit. In the copy the agent's `*_test.go` files under `engine/` and `web/` are
   deleted and `hidden/engine/*` and `hidden/web/*` are copied in, so the agent's tests cannot clash
   with or shadow the hidden ones. Then:
   - `go test -json -vet=off -count=1 -timeout 180s -run '^TestHidden' ./engine`, **without**
     `-race`, because these tests time `Generate`;
   - the same for `./web` with `-timeout 450s` and **with** `-race`. Whether `-race` works is
     decided once, by building an empty program with it, so the decision does not depend on the
     agent's code.

   `-vet=off`, because `go test` otherwise runs its own vet checks first, and one vet finding would
   fail every hidden test of the package although vet is only a soft gate.
5. **Score.** A small Go tool built in the temporary directory reads the `go test -json` events,
   copies the test output to the log, and records each top-level test's result by package. A test
   that reports `fail` at any point stays failed, so a `--- PASS` line printed by the agent's code
   cannot turn a real failure into a pass. `total` is the number of `^func TestHidden` lines in
   `hidden/`; `passed` is the number of those tests, by package, recorded as passed.
   `score = passed / total`, two decimals, computed with `awk`. A package that does not compile
   scores 0 for its tests.
6. **Exit.** 0 only when every gate passed, the score is 1.00 and both hidden test binaries exited
   0; otherwise 1. A binary that exits non-zero after every test passed (a crash, a race report, a
   timeout) is reported as a failed gate.

The temporary directory belongs to the top-level shell and its EXIT trap, so it is removed in the
logging path too (bash 3.2 runs no EXIT trap in the subshell of a pipeline). When `RESULT_JSON` is
set (always, under `bench.py`), all output is also written to `${RESULT_JSON%.json}.check.log`;
the exit code survives the `tee` (`PIPESTATUS`).

**Time.** About 11 s on the reference with a warm Go build cache and about 22 s with an empty one
(the race-instrumented standard library is built once). A compliant generator near the 2 s limit
(about 1.7 s per call, about ten times slower under `-race`) took 537 s in the self-test, and an
engine whose `init` never returns stopped at the budget: `check.sh` reported 753 s, every step at its cap.

## The hidden tests

They test only the contract in `prompt.md`: exported names, routes, status codes, headers, the JSON
shape and the markup hooks (`id="board"`, `data-won`, and `data-row`, `data-col`, `data-region`,
`data-state`, `data-conflict` on the 100 cells). They never depend on error wording, internal
names, an id format beyond the regex, HTML structure beyond the hooks, or an order the contract
leaves open.

### Engine (`hidden/engine/`, package `engine_test`)

| Test | Checks |
|---|---|
| `TestHiddenGenerateDeterministic` | Seeds 0, 1, 2, 42, 2026, −1, −7, `math.MinInt64`, `math.MaxInt64`: two fresh calls return equal puzzles, `.Seed == seed`, and a second process (the test binary run again) returns the same puzzles |
| `TestHiddenGenerateValid` | Seeds 1–30 plus those: `.Seed == seed`, the oracle's layout check passes and `Validate()` returns nil |
| `TestHiddenGenerateUnique` | The same seeds: the oracle counts exactly one solution |
| `TestHiddenGenerateVaried` | Seeds 1–30 give at least 27 distinct layouts, compared after relabelling region ids, as the prompt requires |
| `TestHiddenGenerateSpeed` | Each call for seeds 1–30 finishes in under 2 s |
| `TestHiddenValidate` | Rejects id 10, id −1, only 9 ids used, a region in two parts, and parts touching only diagonally; accepts rows, columns and 2×5 blocks as regions, the forced layout and a valid but unsolvable layout |
| `TestHiddenCountSolutions` | Fixtures with 0, exactly 1 and 479 306 solutions, at limits −1, 0, 1, 2, 5, 1000 and 2^30: `min(count, limit)`, and 0 for a limit of 0 or less |
| `TestHiddenSolve` | Seeds 1–10: `ok`, and the result obeys the rules (and equals the oracle's when the puzzle is unique); the forced fixture gives its solution; the 0-solution fixture gives `ok == false` |
| `TestHiddenCheck` | 14 boards on the 2×5 blocks layout: empty; marks only; one cat; same row; same column; same region; touching orthogonally, diagonally and anti-diagonally; one cat in three conflicts (listed once, sorted); nine correct cats (not won); the full solution (won); the solution plus marks (won); ten cats with a conflict (not won) |

### Web (`hidden/web/`, package `web_test`)

All requests go through `NewHandler().ServeHTTP` with `httptest`, no sockets. Cell POSTs carry
`HX-Request: true`, as htmx sends. Every response except the winning move's must not fire `won`.

| Test | Checks |
|---|---|
| `TestHiddenIndex` | `GET /` is 200 `text/html`, loads both scripts from this server, and has a form (or htmx element) that posts to `/games` |
| `TestHiddenStatic` | Both scripts are 200 with exactly the vendored bytes |
| `TestHiddenCreateGame` | `POST /games` with `seed=42` is 303 to `/games/{id}` (`[A-Za-z0-9_-]{1,64}`); the page is 200 `text/html`, loads both scripts from this server, has `#board` with `data-won="false"` and 100 empty cells whose `data-region` values form the same partition as `engine.Generate(42).Regions` |
| `TestHiddenState` | JSON content type; all six fields with the right JSON types; the id, seed 42, the regions, an empty board, `conflicts` as `[]` (not `null`), `won` false; a game with seed `-9223372036854775808` reports exactly that seed |
| `TestHiddenRandomSeed` | `seed` absent or empty: 303, the JSON seed's `Generate` regions match, and the two games get different ids and different seeds |
| `TestHiddenMoves` | `mark`, `cat`, `empty` (each twice) on one cell: 200 `text/html` with a `#board` fragment of 100 cells showing that state; the JSON and the full page agree; repeating a state changes nothing |
| `TestHiddenConflicts` | Two cats in one row, one column, one region, and touching diagonally: the fragment's `data-conflict` and the JSON `conflicts` match the oracle; clearing them clears the conflicts |
| `TestHiddenWin` | The oracle's solution for seed 7 placed in shuffled order among 12 marks: only the last response fires `won`; then the fragment, the JSON and the page show the win, and four more cell POSTs answer 409 and change nothing |
| `TestHiddenErrors` | 404 for an unknown id on the page, state and cell routes; 400 for row or col `10`, `-1`, `x` and for state `bogus`, `""`, `CAT`, with the board unchanged; 400 for seed `abc`, `1.5`, `9223372036854775808`; 405 for `GET` on a cell route and on `/games`, `POST /`, `DELETE /games/{id}`, `PUT /games/{id}/state` and `POST /static/htmx.min.js` (never HEAD, which the prompt accepts wherever GET is) |
| `TestHiddenIsolation` | Two `NewHandler()` instances share no games; two games in one handler are independent |
| `TestHiddenConcurrent` | 64 goroutines × 20 mark/empty moves, 4 new games (each then read), and 16 goroutines reading the state until those games exist (at least 20 reads each), on one handler at once, all answered correctly; then ten goroutines each own one row, change it at random and leave it in a known state, and the JSON must show exactly that board (a lost update fails). Under `-race`, an unguarded store fails here. It must stay the last test: a `concurrent map writes` crash cannot be recovered |

### The oracle

The tests never use the agent's `Solve`, `Check` or `Validate` to judge another of its functions.
Each package carries its own test-only oracle (`zz_hidden_oracle_test.go`, the same code twice): a
row-by-row bitmask solver that counts solutions up to a limit, a pairwise rule checker, a region
validator, and a relabelling that compares partitions. Every fixture is checked with the oracle
inside the test, and a wrong fixture fails as `bad fixture`, not as the agent's fault. The oracle's
solver is itself checked against OEIS A002464: rows as regions have 479 306 solutions for n = 10.

The fixtures: rows, columns and 2×5 blocks as regions; a *forced* layout in which nine cats of a
known solution get one-cell regions (so it has exactly one solution); and a layout with two
one-cell regions that touch diagonally (valid, but with no solution).

### Robustness

A bad build must fail tests, not crash or stall the package:

- Every call into the agent's code runs in a goroutine with `recover` and a watchdog (`hiddenTry`),
  so a panic or a hang fails that test only. The watchdogs are not performance requirements: 20 s
  for engine calls and ordinary requests, 60 s for requests that create a game and for the tests'
  own `Generate` calls (a 2 s generator runs about 6.5 times slower under `-race`). The only timing
  the tests assert is the contract's: `Generate` under 2 s, in `TestHiddenGenerateSpeed`.
- A test stops at its first watchdog timeout, and the concurrent workers stop at their first
  problem, so a hung build costs one watchdog per test and the packages stay inside their timeouts.
- `Generate` results are cached per seed (package-level test state, keyed by seed), so even a
  generator that takes close to 2 s per call makes about 50 calls in the engine package.
- Markup is read with a small tokenizer: comments, doctypes and the content of `script`, `style`,
  `textarea` and `title` are skipped; end tags close any elements left open inside them, as
  browsers do; attributes may come in any order. A cell is an element nested in `#board` with
  `data-row`, `data-col` and `data-state`; its hook attributes must use double quotes, as the
  prompt asks. Elements outside `#board` are ignored.
- Script URLs are resolved against the page URL and the first `<base href>`, and must stay on this
  server.

### Deliberate readings of the contract

- `TestHiddenSolve` accepts any valid solution and requires equality with the oracle's only when
  the puzzle is unique, so a broken generator does not also fail `Solve`.
- `TestHiddenGenerateVaried` compares layouts after relabelling, so renumbering one partition does
  not count as variety ("at least 27 distinct region layouts" in the prompt).
- `data-region` may number the regions differently from `Regions`; the partition must match.
- `TestHiddenErrors` requires an unchanged board after rejected moves, as the prompt says for any
  4xx response.

## The judge's rubric

`rubric.md` treats the check as the measure of correctness and judges the rest from the diff and
the final message. A 10 needs a check PASS at 1.00, a generator that proves uniqueness with a solver
and is bounded, a pure `engine`, thin handlers, a concurrency-safe store, idiomatic current Go
(`math/rand/v2`, `ServeMux` patterns, `embed`, `html/template`), htmx 4 used as htmx 4, Alpine
holding only UI state, accessible buttons, phone-width layout, real tests and a short accurate final
message. The 7–9 band also covers a check that fails only a soft gate (any check that fails
without lowering the score) with score 1.00. Lower bands cover htmx 2 idioms, structural problems, scores below 0.90 or 0.60,
and hard-gate failures. One or two points come off for unrequested features (timers, hints, sound,
persistence) and leftover scaffolding.

## The reference solution (local only)

`reference/solution/` is a complete build that meets the rubric's 10. It exists to prove the grader:
`check.sh` scores it PASS 1.00. It is not in the repository.

- **`engine/`.** `solve.go` searches row by row with bit sets for used columns and regions; since
  each row holds one cat, cats can only touch in consecutive rows, so only the previous row's column
  is compared. `generate.go` seeds a PCG generator with the seed, places ten valid cats, grows one
  region around each, and then *repairs*: while the solver finds a second solution, it moves one of
  that solution's cat cells into a neighbouring region, carrying along the shortest path to the
  region's edge and any part cut off from the planted cat. The neighbour then holds two of that
  solution's cats, so that solution is gone, and the planted one survives every repair. Repairs and
  attempts are capped, with a fallback layout that is unique by construction, so every seed
  terminates.
- **`web/`.** `ServeMux` method and wildcard patterns (`GET /{$}`,
  `POST /games/{id}/cells/{row}/{col}`, ...), one mutex around the game store, templates and static
  files embedded with `embed`, rendering into a buffer so a template error becomes a 500, and
  `conflicts` encoded as `[]`.
- **`cmd/meowdoku/`.** `-addr` (default `127.0.0.1:8080`), graceful shutdown on SIGINT or SIGTERM.

Measured on the development Mac (darwin/arm64), without `-race`:

- `Generate` over seeds 1–200: p50 0.8 ms, p95 3.6–4.2 ms, max 6–13 ms (three runs).
- Over 20 000 seeds every puzzle came from the first attempt; the fallback was never used.
- Puzzles have about 1.7 one-cell regions on average, and now and then one very large region. A
  variant that balanced region sizes stopped converging, so the reference keeps random repairs.

## htmx 4 traps this case probes

The hidden tests cannot see client-side behaviour; the judge reads it from the diff. These are the
places where htmx 2 habits break, with what the reference does:

| htmx 4 behaviour | Consequence | Reference |
|---|---|---|
| Every response is swapped except 204 and 304, including 4xx and 5xx | A 409 or 400 body replaces the board | `hx-status:4xx:inherited="swap:none"` and the same for `5xx` on `#board` |
| Attributes are not inherited unless marked | `hx-target` on the board does nothing for its buttons | `hx-target:inherited="#board"`, `hx-swap:inherited=...`, `hx-boost:inherited="true"` on `<body>` |
| Events are renamed (`htmx:afterSwap` → `htmx:after:swap`, `htmx:responseError` → `htmx:response:error`) | Listeners on the old names never fire | `x-on:htmx:response:error`, `x-on:htmx:before:request` |
| `HX-Trigger` fires *after* the swap, on the element that sent the request | With an `outerHTML` swap of the board, that button is already detached and the `won` event never reaches the page | `hx-swap="outerMorph"`, which keeps the button (and keyboard focus) |
| A boosted form pushes the final URL into history, also after a 4xx | After an invalid seed the address bar shows `/games`, and a reload answers 405 | `hx-status:4xx="push:false"` on the start form |

Alpine holds only the how-to-play toggle, the win message and a one-line problem message; the next
state of each cell (empty → mark → cat → empty) is rendered by the server into the button's
`hx-vals`.

## Verifying the grader

### Self-test (needs the local `reference/`)

```bash
bash usecases/meowdoku/reference/selftest.sh                       # the default group, check.sh under bash
bash usecases/meowdoku/reference/selftest.sh --bash /bin/bash      # the same under macOS bash 3.2
bash usecases/meowdoku/reference/selftest.sh --group limits        # reproductions of known limits
bash usecases/meowdoku/reference/selftest.sh --group slow          # 5-15 minutes each
bash usecases/meowdoku/reference/selftest.sh reference reference reference   # flakiness reruns
```

For each variant it builds a workspace the way `bench.py` does (copy `workspace/`, overlay the
variant, `git init`, a `base` commit), runs `check.sh` with `RESULT_JSON` unset (except
`reference-logged`) and a private `TMPDIR`, and compares the exit code, the score and the named
failing test or gate. It also fails a variant whose run leaves `check.sh`'s temporary directory
behind. Each mutation must change its file, so a stale pattern cannot silently test the unmodified
reference, and every variant is gofmt'ed, so a soft-gate variant fails on its own gate.

Groups: `default` must match; `limits` reproduces a known limit recorded in `PLAN.md`, with the
result a complete grader would give, which this one does not (marked ≠); `slow` takes minutes.
The *Item* column names the review item a variant reproduces (see `PLAN.md`; `lim` marks a known
limit); the first twelve are the original variants. Results recorded on 2026-10-16 on the final
grader; the default group (62 variants) gave the same verdicts under bash 5.3 and `/bin/bash`
3.2:

| Variant | Item | Expected | Exit | Score | Failed tests; first failed gate |
|---|---|---|---|---|---|
| `untouched` | - | exit 1, =0.00 | 1 | 0.00 | TestHiddenCheck, TestHiddenCountSolutions and 18 more; gate: go vet ./... |
| `reference` | - | exit 0, =1.00 | 0 | 1.00 | — |
| `check-ignores-diagonal` | - | exit 1, <1, TestHiddenCheck | 1 | 0.90 | TestHiddenCheck, TestHiddenConflicts; gate: go test ./... |
| `check-unsorted-duplicates` | - | exit 1, <1, TestHiddenCheck | 1 | 0.95 | TestHiddenCheck; gate: go test ./... |
| `generate-skips-uniqueness` | - | exit 1, <1, TestHiddenGenerateUnique | 1 | 0.95 | TestHiddenGenerateUnique; gate: go test ./... |
| `generate-global-rand` | - | exit 1, <1, TestHiddenGenerateDeterministic | 1 | 0.80 | TestHiddenGenerateDeterministic, TestHiddenCreateGame and 2 more; gate: go test ./... |
| `store-without-mutex` | - | exit 1, <1, TestHiddenConcurrent | 1 | 0.95 | TestHiddenConcurrent |
| `post-after-win-200` | - | exit 1, <1, TestHiddenWin | 1 | 0.95 | TestHiddenWin; gate: go test ./... |
| `htmx-one-byte` | - | exit 1, =0.00, gate:protected files | 1 | 0.00 | gate: protected files changed or missing: web/static/htmx.min.js |
| `third-party-import` | - | exit 1, =0.00, gate:go build | 1 | 0.00 | gate: go build ./... (offline, standard library only) |
| `not-gofmted` | - | exit 1, =1.00, gate:gofmt | 1 | 1.00 | gate: gofmt -l . reports: engine/engine.go |
| `own-tests-removed` | - | exit 1, =1.00, gate:no func Test | 1 | 1.00 | gate: no func Test in: engine/ web/ |
| `vet-miss` | M1 | exit 1, =1.00, gate:go vet | 1 | 1.00 | gate: go vet ./... |
| `fake-pass-lines` | M5 | exit 1, <1, TestHiddenIndex | 1 | 0.45 | TestHiddenConcurrent, TestHiddenConflicts and 9 more; gate: cmd/meowdoku did not answer GET / on the address given by -addr |
| `stdout-no-newline` | M5 | exit 0, =1.00 | 0 | 1.00 | — |
| `row-wrappers` | M4 | exit 0, =1.00 | 0 | 1.00 | — |
| `row-note-outside-board` | M4 | exit 0, =1.00 | 0 | 1.00 | — |
| `markup-in-comment-and-script` | M4 | exit 0, =1.00 | 0 | 1.00 | — |
| `cells-outside-board` | M4 | exit 1, <1, TestHiddenCreateGame | 1 | 0.80 | TestHiddenConflicts, TestHiddenCreateGame and 2 more |
| `hx-trigger-json` | M6 | exit 0, =1.00 | 0 | 1.00 | — |
| `hx-trigger-escaped` | M6 | exit 0, =1.00 | 0 | 1.00 | — |
| `hx-trigger-notice` | M6 | exit 1, <1, TestHiddenWin | 1 | 0.95 | TestHiddenWin; gate: go test ./... |
| `base-href-scripts` | M7 | exit 0, =1.00 | 0 | 1.00 | — |
| `cdn-scripts` | M7 | exit 1, <1, TestHiddenCreateGame | 1 | 0.90 | TestHiddenCreateGame, TestHiddenIndex; gate: go test ./... |
| `region-relabelled` | M9 | exit 0, =1.00 | 0 | 1.00 | — |
| `goenv-goflags` | M10 | exit 0, =1.00 | 0 | 1.00 | — |
| `slow-seed-2100ms` | M11 | exit 1, =0.95, TestHiddenGenerateSpeed | 1 | 0.95 | TestHiddenGenerateSpeed |
| `concurrent-create-unlocked` | M12 | exit 1, <1, TestHiddenConcurrent | 1 | 0.95 | TestHiddenConcurrent |
| `absolute-location` | M14 | exit 0, =1.00 | 0 | 1.00 | — |
| `content-type-no-charset` | M14 | exit 0, =1.00 | 0 | 1.00 | — |
| `reference-logged` | M15 | exit 0, =1.00 | 0 | 1.00 | — |
| `slow-validate` | M19 | exit 0, =1.00 | 0 | 1.00 | — |
| `cmd-removed` | M20 | exit 1, =1.00, gate:cmd/meowdoku | 1 | 1.00 | gate: cmd/meowdoku does not build |
| `post-root-200` | M22 | exit 1, <1, TestHiddenErrors | 1 | 0.95 | TestHiddenErrors; gate: go test ./... |
| `empty-start-page` | M23 | exit 1, <1, TestHiddenIndex | 1 | 0.95 | TestHiddenIndex |
| `json-seed-string` | M24 | exit 1, <1, TestHiddenState | 1 | 0.60 | TestHiddenConcurrent, TestHiddenConflicts and 6 more |
| `json-won-null` | M24 | exit 1, <1, TestHiddenState | 1 | 0.60 | TestHiddenConcurrent, TestHiddenConflicts and 6 more; gate: go vet ./... |
| `cross-process-salt` | M25 | exit 1, <1, TestHiddenGenerateDeterministic | 1 | 0.95 | TestHiddenGenerateDeterministic |
| `negative-limit` | M25 | exit 1, <1, TestHiddenCountSolutions | 1 | 0.95 | TestHiddenCountSolutions; gate: go test ./... |
| `count-capped` | M25 | exit 1, <1, TestHiddenCountSolutions | 1 | 0.95 | TestHiddenCountSolutions; gate: go test ./... |
| `seed-field-wrong` | M25 | exit 1, <1, TestHiddenGenerateValid | 1 | 0.95 | TestHiddenGenerateValid; gate: go test ./... |
| `constant-random-seed` | M26 | exit 1, <1, TestHiddenRandomSeed | 1 | 0.95 | TestHiddenRandomSeed |
| `float-seed` | M26 | exit 1, <1, TestHiddenRandomSeed | 1 | 0.90 | TestHiddenRandomSeed, TestHiddenState; gate: go vet ./... |
| `negative-seed-rejected` | M26 | exit 1, <1, TestHiddenState | 1 | 0.95 | TestHiddenState |
| `web-own-conflicts` | M26 | exit 1, <1, TestHiddenConflicts | 1 | 0.95 | TestHiddenConflicts |
| `stale-page` | M26 | exit 1, <1, TestHiddenMoves | 1 | 0.95 | TestHiddenMoves |
| `won-trigger-on-get` | M26 | exit 1, <1, TestHiddenCreateGame | 1 | 0.85 | TestHiddenCreateGame, TestHiddenMoves, TestHiddenWin |
| `lost-updates` | M26 | exit 1, <1, TestHiddenConcurrent | 1 | 0.95 | TestHiddenConcurrent |
| `single-quoted-hooks` | M27 | exit 1, <1, TestHiddenCreateGame | 1 | 0.80 | TestHiddenConflicts, TestHiddenCreateGame and 2 more |
| `engine-imports-os` | M27 | exit 1, =1.00, gate:engine imports | 1 | 1.00 | gate: engine imports I/O packages: os |
| `text-template` | M27 | exit 1, =1.00, gate:html/template | 1 | 1.00 | gate: web does not use html/template |
| `no-embed` | M27 | exit 1, =1.00, gate:embed | 1 | 1.00 | gate: cmd/meowdoku did not answer GET / on the address given by -addr |
| `engine-uses-fmt` | M27 | exit 0, =1.00 | 0 | 1.00 | — |
| `templates-in-subpackage` | M27 | exit 0, =1.00 | 0 | 1.00 | — |
| `embed-in-static-package` | M27 | exit 0, =1.00 | 0 | 1.00 | — |
| `embed-in-cmd` | M27 | exit 1, <1, TestHiddenStatic | 1 | 0.95 | TestHiddenStatic; gate: go test ./... |
| `both-template-packages` | M27 | exit 0, =1.00 | 0 | 1.00 | — |
| `addr-port-random` | M20 | exit 0, =1.00 | 0 | 1.00 | — |
| `generator-fixed-table` | M2 | exit 1, <1, TestHiddenGenerateVaried | 1 | 0.95 | TestHiddenGenerateVaried |
| `versions-txt-removed` | M8 | exit 0, =1.00 | 0 | 1.00 | — |
| `empty-go-sum` | M21 | exit 0, =1.00 | 0 | 1.00 | — |
| `invalid-state-mutates` | M28 | exit 1, <1, TestHiddenErrors | 1 | 0.95 | TestHiddenErrors |
| `generate-repeat-differs` (limits) | lim | exit 1, <1, TestHiddenGenerateDeterministic | 1 | 1.00 ≠ | gate: go test ./... |
| `slow-generator` (slow) | M3 | exit 0, =1.00 | 0 | 1.00 | — |
| `store-deadlock` (slow) | M16 | exit 1, <1, TestHiddenConcurrent | 1 | 0.70 | TestHiddenConcurrent, TestHiddenConflicts and 4 more; gate: go test ./... |
| `init-hang` (slow) | M17 | exit 1, any | 1 | 0.00 | TestHiddenCheck, TestHiddenCountSolutions and 18 more; gate: go test ./... (stopped: time limit) |

The correct alternatives (expected PASS 1.00) cover row wrappers and other elements with
`data-row`, markup in comments and scripts, a JSON `HX-Trigger` (also with an escaped key),
`<base href>` with relative script URLs, renumbered `data-region`, an absolute `Location`, a
`Content-Type` without charset, stdout written without newlines, a stale `GOFLAGS` in the go env
file, a slow but correct `Validate`, a generator near the 2 s limit, a removed `VERSIONS.txt`, an
empty `go.sum`, and for the structural gates: an engine that uses `fmt`, the templates and their
`embed` in a subpackage of `web`, the scripts embedded by a package in `web/static`, `text/template`
alongside `html/template`, and a command that starts slowly and spawns a helper process (the
`-addr` check must wait for it and kill its whole process group). `embed-in-cmd` is kept as
evidence of a build that cannot pass: `NewHandler()` alone must serve the embedded scripts, and
`//go:embed` cannot reach `web/static` from `cmd/meowdoku`, so it serves copies. Three reruns of the
reference all passed 20/20; earlier, three runs of `lost-updates` (a probabilistic catch) all
caught it.

### Re-grade a workspace by hand

Keep a run's workspace with `--keep-workspaces`; `bench.py` logs its path (`keeping workspace ...`).
`--dry-run --keep-workspaces` gives an untouched one for free. Then:

```bash
WORKSPACE=/path/to/ccbench-xxxx/ws USECASE_DIR="$PWD/usecases/meowdoku" \
  bash usecases/meowdoku/check.sh
```

`USECASE_DIR` must be absolute, because `check.sh` changes into the workspace first. Without
`RESULT_JSON`, nothing is written to `results/`.

### Play the reference (needs the local `reference/`)

```bash
d=$(mktemp -d) && cp -R usecases/meowdoku/workspace/. "$d" && cp -R usecases/meowdoku/reference/solution/. "$d"
(cd "$d" && go run ./cmd/meowdoku)        # http://127.0.0.1:8080; Ctrl-C stops it
```

The reference was played in Chrome on 2026-10-15: a full game to a win by mouse and by keyboard
(Tab, Enter, Space, with focus kept on the cell), conflict highlighting, the win message and the
locked board, a 409 after the win leaving the board untouched, an invalid seed, a new game from the
win panel, 375 px width and dark mode.

## Maintenance

- **Changing the prompt.** Edit the body of `prompt.md` and check that the parser keeps it whole:
  `python3 -c 'import bench; print(bench.load_usecase("meowdoku")["prompt"])'` must print the body
  exactly, without the frontmatter. Keep the contract section and the hidden tests in step.
- **Changing the hidden tests.** Name every top-level test `TestHidden<Name>(t *testing.T)` (the
  score counts `^func TestHidden` lines, so no helper may start that way), prefix identifiers with
  `hidden` and files with `zz_hidden_`, add no `TestMain`, call the agent's code through
  `hiddenTry`/`hiddenCall`/`hiddenDo`, and keep the two copies of the oracle identical. Engine tests
  must not need `-race`; web tests run with it. Keep `TestHiddenConcurrent` last. Watchdogs are
  for hangs only; size them so that one per test still fits the package timeout, and assert no
  timing the prompt does not state. Every game a web test creates costs one `Generate` call, up to
  about 20 s under `-race` for a compliant generator, so count creations against the 450 s web
  timeout. Rerun the self-test afterwards.
- **Changing the time budget.** `BUDGET` (750 s) and the per-step caps are at the top and in the
  body of `check.sh`; keep the total under `bench.py`'s 900 s and the `go test -timeout` values
  under their steps' caps.
- **Updating htmx or Alpine.js.** Download the npm tarball, check it against the registry's
  `dist.integrity`, and extract the one file without npm:

  ```bash
  curl -fsSO https://registry.npmjs.org/htmx.org/-/htmx.org-<version>.tgz
  curl -fsS https://registry.npmjs.org/htmx.org/<version> |
    python3 -c 'import json, sys; print(json.load(sys.stdin)["dist"]["integrity"])'
  echo "sha512-$(openssl dgst -sha512 -binary htmx.org-<version>.tgz | openssl base64 -A)"   # must match
  tar -xzOf htmx.org-<version>.tgz package/dist/htmx.min.js > usecases/meowdoku/workspace/web/static/htmx.min.js
  shasum -a 256 usecases/meowdoku/workspace/web/static/htmx.min.js                            # for VERSIONS.txt
  ```

  For Alpine.js the package is `alpinejs` and the file `package/dist/cdn.min.js`. Update
  `VERSIONS.txt` and the versions named in `prompt.md`. `check.sh` compares against `workspace/`, so
  it needs no change. Results from different library versions are not comparable: note the change
  in `PLAN.md`.
- **Moving to a newer Go.** Change the `go` line in `workspace/go.mod`, the minimum minor version in
  `check.sh` (`"$minor" -lt 27`) and the Go version in `prompt.md`.
- **Calibrating the caps.** Edit the frontmatter of `prompt.md` and log the values in `PLAN.md`.
- **Keep the answers out of `workspace/` and `prompt.md`.** Nothing from `hidden/` or
  `reference/`, and no hint of them, belongs in what the agent receives.

## Limits and what is untested

- Only a paid run reaches real agents, the judge and the caps; none of them has run yet.
- **Judge-only requirements.** No check covers these; only the judge reads them, from the diff and
  the final message: cells as labelled buttons and the empty → mark → cat cycle, keyboard play,
  regions told apart without colour, the win message and locked board, error responses never
  replacing the board in the browser, the phone layout, the htmx 4 idioms, and how substantial the
  agent's own tests are (the gate only finds a `func Test`).
- **Printed results.** A failure always beats a printed `--- PASS` line, but code that prints PASS
  lines for every test and then exits 0 before any test runs (for example in an `init`) would still
  be counted: `go test -json` reports those lines as passes and no test reports a failure. Closing
  that needs a channel the code under test cannot write to; it is not built.
- **Determinism** is checked on the nine special seeds (twice each and in a second process); a
  generator that changed its answer on a repeated call for other seeds would pass (self-test
  `generate-repeat-differs`, in the `limits` group).
- **Harness faults** (no Go 1.27+, no perl) reach the CSV and the judge as FAIL 0.00, like a
  hard-gate failure; only the `.check.log` tells them apart.
- **No process isolation.** A stalled handler's goroutine keeps running inside the hidden test
  binary; the watchdogs bound the wait, not the CPU it uses.
- **The default address** of `cmd/meowdoku` (`127.0.0.1:8080`) is never tried, only a random
  `-addr`.
- `check.sh` runs the agent's code and tests on the grading machine, unsandboxed (see the main
  README's known limits).
- `TestHiddenGenerateSpeed` measures wall time; a heavily loaded machine could slow a borderline
  generator. The reference's slowest call over seeds 1–200 was about 150 times under the limit.
- A watchdog stops waiting for a hung call but cannot stop its goroutine, which keeps running (and
  using CPU) until the test binary exits; a stack overflow in the code under test crashes the
  binary.
- The repository's Python test suite runs on Python 3.8 only in CI; locally it was run on 3.9 and
  3.14.
