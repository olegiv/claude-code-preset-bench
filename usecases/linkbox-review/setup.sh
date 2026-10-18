#!/bin/bash
# Lays the reviewed pull request over the base commit as uncommitted changes, new files marked intent-to-add,
# which is how pr-review-toolkit's review-pr finds a change (git diff --name-only). bench.py runs it in the
# workspace before the agent, so a missing plugin stops the run before it costs anything.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
config=${CLAUDE_CONFIG_DIR:-$HOME/.claude}

python3 - "$config" <<'EOF' || exit 1
import json, os, sys
config, plugin = sys.argv[1], "pr-review-toolkit@claude-plugins-official"
try:
    installed = json.load(open(os.path.join(config, "plugins", "installed_plugins.json")))
    enabled = json.load(open(os.path.join(config, "settings.json"))).get("enabledPlugins", {})
except (OSError, ValueError) as e:
    sys.exit("setup: cannot read the Claude Code plugin settings: %s" % e)
if plugin not in (installed.get("plugins") or installed) or not enabled.get(plugin):
    sys.exit("setup: the %s plugin must be installed and enabled (claude plugin install %s)" % (plugin, plugin))
EOF

git apply --intent-to-add "$here/feature.patch"
# check.sh fails a run that moved HEAD: a commit would hide the review from the judge's diff.
git rev-parse HEAD > "$(git rev-parse --git-dir)/ccbench-base"
