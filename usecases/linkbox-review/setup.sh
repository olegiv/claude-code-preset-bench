#!/bin/bash
# Lays the reviewed pull request over the base commit as uncommitted changes, new files marked intent-to-add,
# which is how pr-review-toolkit's review-pr finds a change (git diff --name-only). bench.py runs it in the
# workspace before the agent. The plugin is declared in prompt.md: bench.py refuses to start without it and
# enables only it for the run.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)

git apply --intent-to-add "$here/feature.patch"
# check.sh fails a run that moved HEAD: a commit would hide the review from the judge's diff.
git rev-parse HEAD > "$(git rev-parse --git-dir)/ccbench-base"
