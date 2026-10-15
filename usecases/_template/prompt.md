---
# Copy this folder to usecases/<your-name>/ (folders starting with _ are ignored).
# All keys optional. Defaults: max_turns 40, max_budget_usd 10, timeout_seconds 1800.
max_turns: 40
max_budget_usd: 10
timeout_seconds: 1800
# repo: ~/Projects/some-repo      -> fresh detached `git worktree` of HEAD per run (uncommitted changes are NOT included)
#                                   leave unset to copy ./workspace/ into a fresh temp dir instead (an empty dir if absent)
---
Write the prompt exactly as you would type it into Claude Code.

It is passed verbatim as the -p argument, so multi-line text is fine.
