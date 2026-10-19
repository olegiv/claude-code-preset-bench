#!/bin/bash
# Records the starting commit: check.sh fails a run that moved HEAD, because a commit would hide the ADR from the
# judge's diff (the working tree against HEAD). bench.py runs it in the workspace before the agent.
set -euo pipefail

git rev-parse HEAD > "$(git rev-parse --git-dir)/ccbench-base"
