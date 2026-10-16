# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `bench.py` runs the same prompt through several Claude Code presets, each run in a fresh
  workspace. It records time, turns, tokens, per-model cost (the advisor's share included),
  subagent counts, a deterministic `check.sh` verdict and an optional LLM-judge score, and has
  `list`, `smoke`, `run` and `summarize` commands. Standard-library Python 3.8+.
- Five presets in `configs/`, each with model and effort pinned: `high` (Opus at xhigh, Fable
  advisor, Sonnet subagents), `normal` (Opus at medium), `normal-max` (Opus at max), `light`
  (Sonnet at medium, Opus advisor, Haiku subagents) and `light-force` (`light` with every
  subagent forced onto Haiku).
- `install-launchers.sh` links the presets onto `PATH` as `oclaude-*` launchers, so the
  benchmarked scripts and the daily launchers are the same files.
- Started from inside a Claude Code session, `bench.py` drops that session's variables, so each
  run behaves as if started from a terminal. The presets without FORCE unset
  `CLAUDE_CODE_SUBAGENT_MODEL_FORCE`, so a value exported in your shell cannot leak in.
- An example use case (`example-fix-tests`), a use-case template, `PLAN.md` with the experiment
  design, and the results of a first calibration pass.
- CI: pylint on `bench.py` and the tests; the tests and a compile-and-list check on Python 3.8
  and 3.14.
- A `unittest` suite in `tests/`, with no model calls: parsing and cost accounting, config and
  use-case discovery, the environment clean-up, workspaces and the check runner, timeouts, every
  preset against a fake `claude`, the launcher installer and the command line.
- `CLAUDE.md` and `AGENTS.md` with instructions for coding agents working on this repository.
- A `meowdoku` use case: each arm builds a 10×10 logic-puzzle web game in Go 1.27 with the
  vendored htmx 4.0.0 and Alpine.js 3.17.4. `check.sh` runs 20 hidden Go acceptance tests and
  scores the fraction passed, behind hard gates (go.mod and the two vendored scripts untouched,
  an offline build that admits no third-party modules) and soft gates (gofmt, vet, the agent's own tests, a
  `cmd/meowdoku` that serves `-addr`, an engine without I/O imports, `html/template`, `embed`),
  all inside a 750 s budget with every step in its own process group and every file it writes
  capped at 128 MB. Results come from
  `go test -json`, so a printed PASS line cannot outweigh a failure. `rubric.md` guides the judge.
  Grading needs Go 1.27 or later and perl, and a C toolchain only for `-race`. Its caps are
  provisional. Alpine.js's MIT notice ships next to the vendored file.
  Reference solutions go in `usecases/*/reference/`, which is gitignored.
  `usecases/meowdoku/README.md` documents running it, the grading, the hidden tests, the
  reference solution and maintenance.

### Fixed

- `install-launchers.sh` no longer overwrites a launcher whose `.bak` already exists, or reports a
  backup it did not make. It now checks every launcher first and, while such a `.bak` is in the
  way, refuses and changes nothing.
