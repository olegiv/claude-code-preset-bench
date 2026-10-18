---
# Provisional caps: calibrate with `python3 bench.py run --runs 1 -c light -u linkbox-review`
max_turns: 80
max_budget_usd: 20
timeout_seconds: 2700
---
This repository is linkbox, a small Go HTTP service (see README.md). The working tree holds an uncommitted pull request on top of the last commit, "Add link previews, pagination and CSV export": `git diff` shows it, and its new files are marked intent-to-add.

Review this pull request. Do not fix it.

1. Run the pr-review-toolkit review on it: `/pr-review-toolkit:review-pr comments tests errors types code`. Leave out `simplify`, which edits code.
2. Check what its agents report against the code: drop what is wrong and add what they missed.
3. Write `REVIEW.md` in the repository root: a short summary, then a `## Findings` section with one line per problem, most severe first:

   `- [<severity>] <path>:<line> — <title>: <explanation>`

   `<severity>` is `critical`, `high`, `medium` or `low`; `<path>` is relative to the repository root; `<line>` is one line, in the changed file, where the problem is. For example (not a real finding):

   `- [medium] internal/foo/bar.go:120 — Short title: what is wrong and what it breaks.`

   List each problem once. Only the first 25 findings are read, so leave out style preferences.

Rules:

- Create `REVIEW.md` and nothing else: do not change, add or delete any other file, and do not commit. The grader reads the working tree.
- Finish with a two-sentence verdict on the pull request.
