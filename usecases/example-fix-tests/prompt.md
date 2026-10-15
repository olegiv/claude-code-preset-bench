---
# Per-use-case settings (all optional). Lines starting with # are ignored.
max_turns: 30
max_budget_usd: 5
timeout_seconds: 900
# repo: ~/path/to/a/git/repo   -> each run gets a fresh detached worktree of HEAD instead of workspace/
---
The unit tests in `test_textutils.py` fail. Fix the bugs in `textutils.py` so that
`python3 -m unittest -q test_textutils` passes.

Rules: do not modify the test file, keep the change minimal, and finish with a short
summary of what you changed and why.
