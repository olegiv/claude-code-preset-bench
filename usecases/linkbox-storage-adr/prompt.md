---
# Provisional caps: calibrate with `python3 bench.py run --runs 1 -c light -u linkbox-storage-adr`
max_turns: 60
max_budget_usd: 15
timeout_seconds: 2400
---
This repository is linkbox, a small Go HTTP service (see README.md). The team keeps its notes in `docs/`; start with `docs/brief.md`.

Write the architecture decision record that the brief asks for.

1. Read the brief, the rest of `docs/` and the code, and ground the decision in them: say where a fact comes from when it matters.
2. Write it as `docs/adr/0002-<title>.md`, with a short, lowercase, hyphenated title like ADR 0001's, and in the format of `docs/adr/0001-keep-links-in-memory.md`: the same sections (Status, Context, Decision, Consequences, Alternatives considered), each with text, and at least 200 words in all.
3. Create only that file: do not change, add or delete any other file, and do not commit. The grader reads the working tree.

Finish with two sentences: the decision, and the main reason for it.
