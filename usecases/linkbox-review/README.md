# linkbox-review

UC2 of the benchmark: a pull-request review with the pr-review-toolkit plugin, the team's daily workflow. It is
the use case where subagents certainly run, and where the presets differ most in what they run them on:
`code-reviewer` and `code-simplifier` pin `model: opus`, the toolkit's other agents inherit the main model, and
`light-force` puts all of them on Haiku.

The agent reviews an uncommitted pull request in `linkbox`, a small Go HTTP service written for this use case
(MIT, like the repository, so no model has seen it). The pull request, "Add link previews, pagination and CSV
export", adds about 600 lines in 9 files and carries ten planted problems and five decoys. The agent writes
`REVIEW.md`; the check matches its findings against the answer key, and the judge grades the rest.

## Files

| File | What it is |
|---|---|
| `prompt.md` | Caps (from the calibration pass) and the task |
| `workspace/` | The base service: about 800 lines of Go with tests; `bench.py` commits it |
| `feature.patch` | The pull request; `setup.sh` applies it. It is a patch, not Go files, so the repository's code scanning does not flag the planted problems |
| `setup.sh` | Applies the patch with `git apply --intent-to-add` and records the starting commit in `.git/ccbench-base` |
| `answer-key.json` | The planted problems and decoys, with line windows in the changed files |
| `check.sh` | The grader |
| `rubric.md` | The judge's rubric; it holds the answer key |
| `selftest.sh` | Checks the use case, then grades 19 known reviews |

## Requirements

- The pr-review-toolkit plugin (`pr-review-toolkit@claude-plugins-official`) installed for the user that runs
  `bench.py`. `prompt.md` declares it: `bench.py` refuses to start without it, and enables it, and no other plugin,
  for the run. The use case was written against version `ea10b213686c` (updated 2026-10-16); the plugin updates
  itself, which can change its agents and their models.
- Go 1.27 for the agents, who will build and test the service; git and python3 for the grader.

## The workspace

`bench.py` commits `workspace/`; `setup.sh` applies `feature.patch` on top, leaving it uncommitted with new files
marked intent-to-add. That is how `/pr-review-toolkit:review-pr` finds a change (`git diff --name-only`), and the
judge's diff (the working tree against `HEAD`) then holds the pull request and `REVIEW.md`. Both the base and the
pull request build, pass `go vet` and pass their tests, `-race` included: no planted problem shows up without
reading the code.

The prompt names the review aspects (`comments tests errors types code`): without them, `review-pr` would also run
`code-simplifier`, which edits code, and that fails the hard gate.

## How `check.sh` grades

1. Hard gates (0.00): every file of the pull request must be unchanged, `HEAD` must still be the commit
   `setup.sh` recorded (the judge's diff would not show committed work), and `REVIEW.md` must exist.
2. Findings are lines `- [severity] path:line — title: explanation` (a backticked location, a line range and `-`
   for the dash are accepted). Only the first 25 are read, and each counts at the first line it cites, so citing
   whole files or spreading findings evenly gains little: the self-test's evenly spaced 25 findings score 0.10.
3. A finding whose line falls in a planted problem's window finds it (once); one in a decoy's window flags the
   decoy. Score = max(0, problems found − decoys flagged) / 10.
4. Soft gates (the check fails, the score stands): files added besides `REVIEW.md` (built binaries and test
   output excepted), no finding in the format, more than 25 findings, and, when `bench.py` passes the run's result
   JSON, no completed pr-review-toolkit agent. An agent killed when the session ended does not count; as the run counts completions over all agent types, a toolkit agent counts as completed only when more agents completed than agents of other types started.
5. PASS needs a score of at least 0.60 and no failed gate; a review rarely finds everything, so the bar is not
   1.00 as in the other use cases.

## Planted problems and decoys

| # | Area | Problem |
|---|---|---|
| 1 | security | SSRF: the preview fetcher requests any saved URL, internal addresses included |
| 2 | resources | The response body is not closed on a non-200 status |
| 3 | concurrency | The preview cache map is used without a lock from concurrent goroutines |
| 4 | tests | The timeout test never reaches the timeout |
| 5 | security | Path traversal through the export name (`/../../x.csv`), which checks only `.csv` |
| 6 | authorization | `?owner=` lets anyone export anyone's links |
| 7 | logic | Pages are documented from 1 but handled from 0, so page 1 skips the first page |
| 8 | error handling | The preview handler discards the store's error and answers 200 |
| 9 | comments | "Kept for a day" next to a 24-minute TTL |
| 10 | type design | `ListByOwner` returns pointers into the store, against its documented copies |

Decoys, where any finding is wrong: the export-cleanup goroutine (it stops on shutdown), rune-based title
truncation, the cleanup's `os.Remove` (only prefixed regular files it listed), the deferred `Close` next to a
checked one, and ignored `csv.Writer.Write` errors (the sticky error is checked after `Flush`). Windows are placed
so that a fair finding about a problem cannot land on a decoy, and `selftest.sh` asserts that each anchor string
lies in its window and that no windows overlap. The code around the decoys leaves nothing else to find there:
`WriteCSV` escapes cells that a spreadsheet would read as formulas, `Cleanup` carries on past a file it cannot
remove, and the cleanup goroutine also sweeps at startup. Calibration runs lost 0.10 for each of these to valid
findings on decoy lines before they were fixed.

## Self-test

`bash selftest.sh` (about 10 seconds) first checks the anchors and windows and that the base and the pull request
build, vet and test, then grades 19 generated reviews; `CHECK_BASH=/bin/bash` uses macOS's bash 3.2.

| Variant | Verdict |
|---|---|
| `perfect` (one finding per problem) | PASS, 1.00 |
| `perfect-toolkit-used` / `-killed` / `-killed-explore-done` / `no-toolkit` (fake result JSON) | PASS / soft-gate FAIL / soft-gate FAIL / soft-gate FAIL, 1.00 |
| `seven`, `half` | PASS 0.70, FAIL 0.50 |
| `perfect-plus-two-decoys` | PASS, 0.80 |
| `all-decoys`, `duplicates` | FAIL 0.00, FAIL 0.10 |
| `spam-ranges`, `spam-evenly-spaced` | FAIL 0.00, FAIL 0.10 |
| `26-findings`, `wrong-format` | soft-gate FAIL 1.00, FAIL 0.00 |
| `no-review`, `code-edited`, `committed` | hard gate, 0.00 |
| `extra-file`, `binary-ignored` | soft-gate FAIL 1.00, PASS 1.00 |

## Known limits

- The answer key is in this public repository and the agent runs unsandboxed, as in the other use cases.
- A real problem outside the key earns nothing from the check; the judge credits it.
- A review that cites the right line for the wrong reason still counts; the judge sees the explanation.
- Ceiling: in calibration every preset found nine or ten of the ten problems, so the check barely separates them;
  the judge and the cost decide.
- `bench.py` counts Opus usage in `light` as its advisor's cost; on UC2, `code-reviewer`'s Opus usage lands there
  too, so read `light`'s `per_model_cost_usd` with that in mind.
