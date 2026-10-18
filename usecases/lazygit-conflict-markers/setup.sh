#!/bin/bash
# Fills the empty workspace with lazygit at the parent of upstream PR #5902 and makes sure the grader's
# image is present. bench.py runs it in the workspace before the agent, outside the agent's time.
set -euo pipefail

. "$(dirname "$0")/pins.sh"

# A local cache keeps one copy of the commits, so only the first workspace downloads them; it is refreshed
# when the pins in pins.sh change.
if [ "$(git -C "$CACHE" rev-parse -q --verify "$BASE_REF^{commit}" 2>/dev/null)" != "$BASE" ] ||
   [ "$(git -C "$CACHE" rev-parse -q --verify "$REFERENCE_REF^{commit}" 2>/dev/null)" != "$REFERENCE" ]; then
  mkdir -p "$(dirname "$CACHE")"
  [ -d "$CACHE" ] || git init -q --bare "$CACHE"
  git -C "$CACHE" fetch -q --depth 1 "$REPO_URL" "+$BASE:$BASE_REF" "+$REFERENCE:$REFERENCE_REF"
fi
[ "$(git -C "$CACHE" rev-parse "$BASE_REF")" = "$BASE" ] || { echo "setup: cache $CACHE holds the wrong base commit" >&2; exit 1; }

# The workspace is an empty repository with one empty commit: replace it with the base tree.
git fetch -q --depth 1 "$CACHE" "$BASE_REF"
git reset -q --hard FETCH_HEAD
# Upstream's AGENTS.md (CLAUDE.md points to it) prescribes commits, just recipes and a gopls MCP server
# that benchmark runs do not have. Removing them in a commit keeps the judge's diff to the agent's work.
git rm -q AGENTS.md CLAUDE.md
git -c user.name=ccbench -c user.email=ccbench@local -c commit.gpgsign=false \
  commit -q --no-verify -m "Remove the upstream agent instructions"

# check.sh grades in this image with --pull never, so pull it now, while the network is allowed.
command -v docker >/dev/null 2>&1 || { echo "setup: Docker is required to grade this use case" >&2; exit 1; }
docker image inspect "$IMAGE" >/dev/null 2>&1 || docker pull -q "$IMAGE" >/dev/null
