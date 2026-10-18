# lazygit-conflict-markers

UC1 of the benchmark: an exploration-heavy change in a real codebase. Each run starts from
[lazygit](https://github.com/jesseduffield/lazygit) (MIT, about 150 000 lines of Go, dependencies vendored) at the
commit before upstream PR [#5902](https://github.com/jesseduffield/lazygit/pull/5902), "Honor the
conflict-marker-size gitattribute" (merged 2026-08-08, fixing issue
[#4367](https://github.com/jesseduffield/lazygit/issues/4367)). The agent gets the bug in its own words, without file
names, and must find where conflicted files are loaded, where conflict markers are recognized (three call sites)
and the check that auto-stages files it takes for resolved. Upstream's fix touches 8 files in 6 packages.

## Files

| File | What it is |
|---|---|
| `prompt.md` | Caps (provisional) and the task |
| `pins.sh` | The upstream commits (base `5dec89a`, reference `f162dc5`), the cache path and the grader's image |
| `setup.sh` | Fills each workspace with the base commit and pulls the image |
| `check.sh` | The grader |
| `hidden/` | The hidden integration tests, and lazygit's licence, which covers the two taken from the PR and the helper |
| `rubric.md` | The judge's rubric |
| `selftest.sh` | Grades known variants of the workspace and compares the verdicts |

## Requirements

- Docker, on the machine that runs `bench.py`: the grader builds and tests in a container. Without it, `setup.sh`
  fails before the agent starts, and `check.sh` reports a harness fault with 0.00.
- Network the first time `setup.sh` runs: it fetches the base and reference commits into
  `${XDG_CACHE_HOME:-~/.cache}/ccbench/lazygit.git` (about 5 MB; `CCBENCH_CACHE` moves it) and pulls
  `golang:1.27.2` pinned to its multi-platform digest. Later workspaces use the cache.
- Go 1.25 or later and git on the host, for the agent: it builds and tests lazygit there. Upstream's integration
  tests also pass on macOS (16 of 16 conflict tests, git 2.56).
- python3, which `bench.py` needs anyway: `check.sh` uses it to read `go test -json`.

`bench.py run --dry-run` builds each workspace too, so a dry run of this use case also runs `setup.sh`.

## The workspace

`bench.py` creates an empty repository; `setup.sh` fetches the base commit into it (shallow, so the fix is not in
the history), deletes upstream's `AGENTS.md` and `CLAUDE.md` and commits that. Upstream's agent guide prescribes
commits, `just` recipes and a gopls MCP server that runs do not have, and a committed change would leave the judge
an empty diff, since `bench.py` diffs the working tree against `HEAD`. The prompt says not to commit.

`results/runs.csv` records no base commit for this use case (`workspace_kind` is `copy`): a run's lazygit commit is
`BASE` in `pins.sh`, as of the benchmark commit that produced the row.

## How `check.sh` grades

1. Harness checks: Docker and its daemon, the pinned image, python3 and the cached base commit. A missing one is a
   harness fault: `FAIL: harness: …` and 0.00, which, like meowdoku's, only the log tells apart from a hard gate.
2. Hard gate: `go.mod`, `go.sum` and `vendor/` must equal the base commit, so the offline build tests the agent's
   code and nothing it fetched.
3. The grading tree is a copy of the workspace in which `pkg/integration/{tests,components,clients}` are replaced
   by upstream's at the base commit plus `hidden/`. The agent's own integration tests are dropped: they can neither
   collide with the hidden ones nor stand in for them. `pkg` and `pkg/integration` must be real directories (a
   hard gate), since replacing them would otherwise follow a link out of the grader's temporary directory.
4. In a container (`--network none --pull never`, nothing mounted, stopped after 360 s): `go generate` of the test
   list, `go build ./...` (hard gate), then the soft gates gofmt, `go vet ./...` and `go test -short -timeout 60s
   ./...` (the workspace's unit tests), and all 517 integration tests with `go test -json -timeout 4m`. Agent code
   runs in there, so of the results that come back, only regular files are kept.
5. A test that hangs ends the run at that timeout and leaves tests without a result; the suite then runs once more
   in a new container. A test left without a result after that counts as failed.
6. Score: the fraction of the 5 acceptance tests that pass. The other 512 integration tests are regression tests:
   one that fails gets one rerun in a new container (upstream runs each test once, `MaxAttempts: 1`), and a second
   failure fails the check without lowering the score. Acceptance failures are never rerun.

The whole check takes about half a minute; its worst case (360 + 240 + 240 s) stays under `bench.py`'s 900 s. The
first full self-test had one grading container stall with no results, which is why a hang is now caught inside
the container; reruns did not repeat it.

## Hidden tests

Acceptance tests fail at the base commit and pass at the reference (checked five times each way):

| Test | Origin | What it checks |
|---|---|---|
| `conflict_marker_size_resolve` | PR #5902 | A file with 32-character markers opens in the merge-conflict editor and picking a hunk resolves it |
| `conflict_marker_size_not_auto_staged` | PR #5902 | A refresh does not stage the file while its conflicts remain; once they are resolved, lazygit offers to continue |
| `conflict_marker_size_short_markers_are_content` | ours | A seven-character marker block in such a file is content: only the real hunk is offered, and the block survives |
| `conflict_marker_size_from_pattern` | ours | A length of 12 set by a `*.md` pattern works, which catches hard-coded lengths and literal parsing |
| `conflict_marker_size_resolve_all_hunks` | ours | Both 32-character hunks resolve and lazygit then offers to continue the merge |

`conflict_marker_size_other_files_unaffected` (ours) passes at both commits, so it is a regression test: a conflict
in a file the attribute does not cover keeps the default markers.

Upstream's unit tests from the PR are not used: they pin internal signatures (`findConflicts(content, markerSize)`)
and the exact git calls of the file loader, which a correct fix built differently would fail. The existing
`file_loader_test.go` fakes git's output command by command, so a fix that adds a git call must update it, and a
failure there is "didn't finish the job", not grader noise.

## Self-test

`bash selftest.sh` (about 8 minutes) builds each variant's workspace as `bench.py` does and runs `check.sh` on it;
`CHECK_BASH=/bin/bash` uses macOS's bash 3.2. `bash selftest.sh reference` runs one variant. On 2026-10-18 all 18
gave the expected verdict under bash 5.3 and under `/bin/bash` 3.2.

| Variant | What it is | Verdict |
|---|---|---|
| `untouched` | The workspace as `setup.sh` leaves it | FAIL, 0.00: acceptance 0/5 |
| `reference` | Every file upstream's PR changed, its tests included | PASS, 1.00 |
| `reference-committed` | The reference, committed | PASS, 1.00 (the judge would see an empty diff) |
| `reference-agent-tests-broken` | The reference plus an integration test of the agent's that does not compile | PASS, 1.00: the grader drops it |
| `reference-no-test-updates` | The reference with the PR's unit-test changes undone | FAIL, 1.00: go vet and the unit tests (the old `findConflicts` signature, the file loader's fake git calls) |
| `reference-gofmt-miss` | The reference with one misformatted line | FAIL, 1.00: gofmt |
| `hardcoded-32` | 32 for every file that sets the attribute, whatever its value | FAIL, 0.80: `from_pattern` |
| `auto-stage-default` | The auto-staging check left on the default length | FAIL, 0.60: `not_auto_staged`, `short_markers_are_content` |
| `unset-means-32` | 32 also for files without the attribute | FAIL, 1.00: 32 other integration tests and a unit test break |
| `spins-on-pattern` | lazygit loops forever on a length of 12 | FAIL, 0.80: the test driver gives up, so `from_pattern` fails rather than hangs |
| `first-run-times-out` | The reference, with the first integration run stopped after 2 s | PASS, 1.00, after one rerun of the suite |
| `vendor-changed`, `go-mod-changed` | A vendored file, `go.mod` | FAIL, 0.00: hard gate |
| `build-broken` | A syntax error | FAIL, 0.00: hard gate |
| `unit-test-hangs` | The reference plus a unit test that never returns | FAIL, 1.00: the unit tests stop at their timeout and the integration tests still run |
| `result-symlink` | The reference plus a unit test that swaps a result log for a link to a host file | FAIL, 1.00, and the host file's content is not printed |
| `pkg-integration-symlink` | `pkg/integration` replaced by a link to a directory outside the workspace | FAIL, 0.00: hard gate, and the directory is untouched |
| `docker-down` | A `docker` that fails, first on `PATH` | harness fault, 0.00 |

## Known limits

- The agent runs unsandboxed with network access, so it can look up PR #5902 upstream. The prompt asks it not to;
  nothing detects it. This is the same accepted limit as meowdoku's public hidden tests.
- Agent code runs in the grading container, so it could print `go test -json` events for the hidden test names, or
  write the grader's result files, and fake a pass; closing that needs a results channel the code under test cannot
  write. Links among the results are dropped, so it cannot make the grader print a host file.
- The grader runs Linux in a container while agents work on macOS; upstream's CI runs the integration tests on
  Linux only.
- `setup.sh` needs network on a machine's first run, and the grader needs Docker.

## Maintenance

To move to another upstream commit, change `BASE` and `REFERENCE` in `pins.sh` (`setup.sh` refreshes the cache when
they no longer match), check that the hidden tests still fail at the base and pass at the reference
(`selftest.sh`), and record the change in `PLAN.md`. To update the image,
pin the new tag to its index digest (`docker buildx imagetools inspect golang:<tag>`).
