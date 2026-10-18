---
# Provisional caps: calibrate with `python3 bench.py run --runs 1 -c light -u lazygit-conflict-markers`
max_turns: 150
max_budget_usd: 30
timeout_seconds: 3600
---
This directory is a checkout of lazygit, the terminal UI for git, written in Go. Fix the bug below.

## The bug

Git's `conflict-marker-size` attribute, set in a `.gitattributes` file (for example `*.md conflict-marker-size=12`), makes git write conflict markers of that many characters instead of the default seven. People set it for files whose normal content contains lines that look like conflict markers, such as documentation about merging.

Lazygit ignores the attribute and recognizes only seven-character markers. In a file whose markers are longer:

- the merge-conflict editor doesn't open, so the conflict can't be resolved in lazygit;
- lazygit takes the file for resolved while its conflicts are still in it, stages it on the next refresh and offers to continue the merge;
- seven-character marker lines that belong to the file's content are mistaken for a conflict.

## Required behavior

For each conflicted file, lazygit uses the marker length that git used for that file, as git itself resolves the attribute (patterns, `.gitattributes` files in subdirectories and the other places git reads attributes from), and seven when the attribute is not set:

- The file opens in the merge-conflict editor, which shows its hunks, and picking a side resolves them.
- A file that still contains a conflict is never taken for resolved: it isn't staged automatically, and lazygit offers to continue the merge or rebase only once every conflict is really resolved.
- Marker-like lines of any other length are ordinary content and stay as they are.
- Files that the attribute doesn't cover behave exactly as before.

## Constraints

- Don't change `go.mod`, `go.sum` or `vendor/`: no new dependencies.
- Don't commit. The grader reads your changes from the working tree.
- Keep the existing tests passing, changing them only where the new behavior requires it, and add tests for the change.
- Work it out from the code in this directory, without looking up how upstream lazygit handled this.
- If you start lazygit or anything else that keeps running, stop it before you finish.
- Finish with a short summary: where the marker length comes from, which parts of lazygit use it, and what you tested.
