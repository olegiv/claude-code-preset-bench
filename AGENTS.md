# Repository instructions

This repository benchmarks Claude Code presets, and the preset scripts double as the owner's daily
launchers. Read `CLAUDE.md` for the structure, commands, checks and conventions; they apply to
every agent.

## Before running anything

`python3 bench.py smoke` and `python3 bench.py run` call models and cost real usage. Run them only
when asked, and check changes with `python3 bench.py list` and `python3 bench.py run --dry-run`.

## Behavior verification before publication

Before presenting a change as ready for commit approval or push:

1. Map each changed behavior to its failure scenario and direct evidence, plus a normal case that
   must still work.
2. Trace the change through its callers and consumers. A config feeds both the launcher installed
   from it and the benchmark arm it defines; a `bench.py` change reaches the `list`, `smoke`, `run`
   and `summarize` paths that call it.
3. Exercise it without model calls where possible: `bash -n`, a fake `claude` placed first on
   `PATH` that prints its arguments and environment, `bench.py list` and `--dry-run`. Record
   anything left unexecuted, such as a path only a paid run reaches, and do not claim it was tested.
4. Report the baseline checks from `CLAUDE.md` (pylint, and compile and `list` on Python 3.8 and a
   current version) separately from the checks of the changed behavior.
5. Bind the evidence to the exact tree being committed, and refresh it after any further change.

Pure wording or formatting edits need only a structural check.
