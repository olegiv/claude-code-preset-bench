# linkbox-storage-adr

UC3 of the benchmark: a document, not code. The agent writes an architecture decision record for linkbox, the
small Go service of UC2, choosing how it stores links from now on. It tests whether the advisor and a stronger
main model add anything when there is nothing to compile, and whether an agent reads the materials closely enough
to reach the one decision that fits them all.

## Files

| File | What it is |
|---|---|
| `prompt.md` | Caps (provisional) and the task |
| `workspace/` | linkbox's base service (copied from `linkbox-review/workspace/`) plus the team's notes in `docs/`; `bench.py` commits it |
| `setup.sh` | Records the starting commit in `.git/ccbench-base` |
| `check.sh` | The gates; the judge grades the content |
| `rubric.md` | The judge's rubric: the facts in the materials, the defensible decision, ten points to check, bands |
| `reference-adr.md` | A reference ADR, not given to the agent; the self-test's passing variant |
| `selftest.sh` | Checks the use case, then runs the check on 17 known workspace states |

## The materials

`docs/brief.md` asks for ADR 0002 in the format of `docs/adr/0001-keep-links-in-memory.md`. The rest of `docs/`
holds the facts, spread over short notes as a team keeps them:

| File | What it contributes |
|---|---|
| `adr/0001-keep-links-in-memory.md` | In memory because linkbox was a prototype on one VM, as a single instance (out of date) |
| `ops/deployment.md` | Platform policy of at least two replicas, no session affinity, up to four instances during a deploy, ephemeral disks, no persistent volumes, health checks, `CGO_ENABLED=0` |
| `ops/platform-services.md` | Managed PostgreSQL (20 connections on the small plan, retention 7 to 35 days, default 35), Redis as a cache tier, object storage without conditional writes; no self-run databases |
| `metrics.md` | A small load, and a p99 target of 100 ms |
| `compliance.md` | EU only, erasure within 30 days including backups, encryption, backups required |
| `support-tickets.md` | Symptoms that only several instances, each with its own memory, explain |
| `team.md` | Standard library only; a week's security review for a new module; CI can start a database |

The traps: the lead's brief leans towards SQLite, and ADR 0001 says linkbox runs as a single instance. Only the
deployment notes show that it has run as two to four instances since July, which rules out every per-instance
store, explains the tickets, and leaves the platform's managed PostgreSQL as the one store that fits every
constraint. A good ADR then follows the consequences into the code: the `l<n>` ID counter, the store's API
without `context` or errors, a connection pool sized for four instances, backup retention under 30 days, and
migrations that tolerate concurrent starts.

## How it is graded

`check.sh` passes a run (score 1.00) when it added exactly one file, `docs/adr/0002-<lowercase-title>.md`, with
ADR 0001's sections (Status, Context, Decision, Consequences, Alternatives considered; the last may be called
Alternatives or Options considered), each with text, at least 200 words in all; changed or deleted no file of the
workspace; added no symlink, whose target the judge's diff would not show; and did not commit, since `bench.py`
shows the judge the working tree against `HEAD`. Any failed gate scores 0.00. Built binaries and test output are
ignored.

The judge scores the content from 0 to 10 with `rubric.md`, which holds the facts the agent read: one point each
for the decision, the replica diagnosis, the reasons against SQLite and the other alternatives, IDs, the store's
API, the connection pool, compliance, schema and migrations, and the rollout. A store other than the managed
PostgreSQL scores at most 3; a generic "PostgreSQL because it is durable" ADR lands in the middle.

## Self-test

`bash selftest.sh` (about 10 seconds) first checks that every fact the rubric cites is stated in the file it names
and that the workspace builds, vets and tests, then runs the check on 17 workspace states;
`CHECK_BASH=/bin/bash` uses macOS's bash 3.2.

| Variant | Verdict |
|---|---|
| `perfect` (the reference ADR), `options-heading`, `binary-ignored` | PASS, 1.00 |
| `missing-section`, `empty-section`, `too-short` | FAIL, 0.00 |
| `no-adr`, `wrong-number`, `uppercase-title`, `wrong-dir`, `two-adrs` | FAIL, 0.00 |
| `extra-file`, `code-edited`, `adr0001-edited`, `symlinked-doc`, `symlinked-adr`, `committed` | FAIL, 0.00 |

## Known limits

- The judge alone measures quality; its scores vary more between runs than a deterministic check's.
- The rubric and the reference ADR are in this public repository, and the agent runs unsandboxed, as in the other
  use cases.
- linkbox's code is a copy of UC2's base service: a change to one does not reach the other.
