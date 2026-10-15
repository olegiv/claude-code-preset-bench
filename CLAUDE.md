# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

Follow the behavior-verification gate in `AGENTS.md` before presenting a change as ready for commit
approval or push.

## Repository overview

`bench.py` benchmarks Claude Code presets (model, effort, advisor, subagent model) against each other.
It runs the same prompt through several command lines ("configs"), several times each, in a fresh
workspace every time, and records time, turns, tokens, per-model cost, a deterministic `check.sh`
verdict and an optional LLM-judge score. The configs double as the owner's daily launchers:
`install-launchers.sh` links them onto `PATH` as `oclaude-*`.

`README.md` documents usage, `PLAN.md` the experiment and its log, `CHANGELOG.md` the changes.

## Layout

| Path | What it is |
|---|---|
| `bench.py` | The harness: `list`, `smoke`, `run`, `summarize`. Standard library only |
| `configs/*.sh` | One preset per file: `exec claude` with the preset's flags and environment, plus `"$@"` |
| `install-launchers.sh` | Links the configs onto `PATH` as `oclaude-<name>`, keeping replaced regular files as `.bak` |
| `usecases/<name>/` | `prompt.md` (frontmatter and prompt), `workspace/`, `check.sh`, `rubric.md`, optional `setup.sh`; folders starting with `_` are ignored |
| `results/runs.csv`, `results/summary.md` | Append-only run log and its summary; `results/raw/` is gitignored |
| `.pylintrc`, `.github/workflows/pylint.yml` | Lint configuration and CI |

## Commands

```bash
python3 bench.py list                                # configs and use cases; no model calls
python3 bench.py run --dry-run --runs 1 --no-judge   # print the exact commands; no model calls
python3 bench.py smoke                               # one 1-turn call per config plus a judge call (costs usage)
python3 bench.py run --runs 3 -u <usecase>           # the real experiment (costs usage)
python3 bench.py summarize                           # tables on stdout and results/summary.md
pylint bench.py                                      # the CI lint; .pylintrc is picked up automatically
```

`smoke` and `run` call models and cost real usage. Run them only when asked; check changes with
`list` and `--dry-run`.

## Checks

There is no test suite. Before a commit, check the exact tree being committed:

- `pylint bench.py` exits 0, with pylint 4.1.3 as in CI (it needs Python 3.10 or later).
- `bench.py` compiles and `python3 bench.py list` works on the oldest Python you have and on a
  current one. CI runs both on Python 3.8 and 3.14.
- `bash -n` passes on every changed shell script.
- For harness changes, `python3 bench.py run --dry-run --runs 1 --no-judge` succeeds. It creates
  and removes a workspace but calls no model.

## Conventions

- **Python 3.8+, standard library only.** The README promises it and CI checks it: no third-party
  imports, and no 3.9+ features such as `str.removeprefix`, `dict | dict` or `list[int]`
  annotations.
- **Style.** %-formatting throughout, lines up to 180 characters, no per-function docstrings;
  `.pylintrc` encodes this. Keep pylint clean, and disable a message inline only with a reason.
- **Configs.** Every config pins `--model` and `--effort`, ends with `exec claude … "$@"`, and, unless
  it sets FORCE, runs `unset CLAUDE_CODE_SUBAGENT_MODEL_FORCE` so a value exported in the shell cannot
  leak in. Pairs of arms differ in one factor (`normal`/`normal-max`: effort; `light`/`light-force`:
  FORCE). Keep them that way, or record the change in `PLAN.md`.
- **Launchers are live.** Installed launchers are symlinks to `configs/*.sh`, so editing a config
  changes the owner's daily `oclaude-*` command at once, as well as a benchmark arm.
- **Subagent models.** `CLAUDE_CODE_SUBAGENT_MODEL` reaches only agents without a `model:` of their
  own. `CLAUDE_CODE_SUBAGENT_MODEL_FORCE` (any non-empty value) puts every subagent on that model,
  `model: inherit` included; only forks and skills with `model: inherit` keep the main model.
- **Environment.** `bench.py` strips variables that would override a config (`STRIP_VARS`) and a
  parent Claude Code session's variables (`SESSION_VARS`). Log only their names: one is a token.
- **Results.** `results/runs.csv` is append-only: never edit or rewrite its rows. Raw outputs stay in
  the gitignored `results/raw/`. Record findings and design changes in the log at the end of
  `PLAN.md`.
- **Public repository.** Keep employer- or client-specific material (private repositories, internal
  documents, hostnames, credentials) out of use cases, prompts, results and commit messages.
- **Changelog.** Add each user-visible change to `CHANGELOG.md` under `[Unreleased]`, in the Keep a
  Changelog format.
- **Commits.** Imperative subject of at most 50 characters, a body wrapped at 72 that explains why,
  and no AI attribution trailers.
