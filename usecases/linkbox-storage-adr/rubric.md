The agent was asked to write ADR 0002 for linkbox, a small Go HTTP service that keeps its users' links in memory: how linkbox stores links from now on, in the format of ADR 0001 (sections Status, Context, Decision, Consequences, Alternatives considered), grounded in the team's notes in `docs/` and in the code, without changing anything else. The diff you see holds the ADR. The deterministic check verified only that exactly one ADR was added with those sections and that nothing else changed; it says nothing about the content, which you grade. The agent read materials you do not see; the facts below are what they say. Do not reward length.

## What the materials say

- Brief (team lead, 2026-10-16): deploys and restarts wipe all links; product wants durable storage this quarter; mark the ADR Proposed; "my gut says SQLite: one file next to the binary, no new infrastructure, our load is tiny".
- ADR 0001 (2026-03-02): in memory because linkbox was a prototype running as a single instance on the team's VM; "revisit when people rely on their links, or when linkbox needs more than one instance".
- `ops/deployment.md` (2026-07-14): linkbox moved to the container platform; platform policy is at least two replicas, no opting out; round-robin load balancer without session affinity; rolling deploys run up to four instances at once; deploys two or three times a week; container filesystems are ephemeral and the platform offers no persistent volumes to applications; `/healthz` every 10 s, three failures restart the replica; builds use `CGO_ENABLED=0`.
- `ops/platform-services.md`: teams may not run databases of their own. Managed PostgreSQL 16 (standby in another zone, about 30 s failover; small plan with at most 20 client connections, medium with 100; encrypted at rest; daily snapshots and point-in-time recovery, retention settable from 7 to 35 days, default 35; `eu-west` or `us-east`; about three working days to provision). Managed Redis: a cache tier, no persistence, evictions, no backups. Object storage: no conditional writes, 20 to 80 ms per request, listing 1,000 keys per page.
- `metrics.md`: 4 to 6 requests per second at peak, 95% reads, about 3,500 new links a day, users tripling within a year, target p99 under 100 ms.
- `compliance.md`: links are personal data; EU regions only (`eu-west`, `eu-central`); erase a closed account's data within 30 days, backups included; encrypted at rest; backups that can restore after an operator's mistake.
- `support-tickets.md`: links gone after a deploy (#4127); a link appears, disappears and comes back on reload (#4188); `GET /links/l7` returns one link, then another (#4201); 14 links on a laptop, 9 on a phone (#4260); deleted links come back (#4302).
- `team.md`: two Go developers; standard library only, and a third-party module needs a security review of about a week; CI can start a database container next to the tests.
- Code: `store.Store` is a map behind a mutex; IDs are `fmt.Sprintf("l%d", s.seq)` from a counter in the process, which restarts at `l1` and differs per instance; the newest-first order breaks ties with `compareIDs`, which relies on that `l<n>` format; `Add`, `Get`, `Delete` and `List` take no `context.Context`, and `List` returns no error; `api.Server` holds a concrete `*store.Store`.

## The defensible decision

The platform's managed PostgreSQL (the small plan is enough), in `eu-west`. Every alternative breaks a stated constraint: SQLite and bbolt are a file per instance, so two to four instances keep diverging (today's bug), and the file is lost with the container since there are no persistent volumes; Litestream or LiteFS allow one writer and need a volume; Redis has no persistence or backups; object storage has no conditional writes (concurrent instances lose updates) and listing a user's links object by object at 20 to 80 ms misses the 100 ms target; a database of the team's own is not allowed; sticky sessions do not exist on the load balancer, and memory is still wiped by deploys.

## How to score

Check the ADR against these points; each is worth about one point. Credit a point only when the ADR states it in terms of linkbox, not as generic advice.

1. Decision: managed PostgreSQL from the platform, justified by the constraints above.
2. Diagnosis: the tickets are explained by several instances each holding their own links (no session affinity), not only by restarts; ADR 0001's single-instance premise no longer holds.
3. SQLite, the lead's suggestion, rejected for the right reasons: per-instance files with at least two replicas, and ephemeral disks without persistent volumes.
4. The other alternatives rejected for the right reasons (Redis, object storage, a self-run database; Litestream or LiteFS if mentioned).
5. IDs: the in-process `l<n>` counter cannot stay (it collides between instances and restarts at `l1`); IDs come from the database or are random; the tie-break in ordering is adapted.
6. Store API: the methods need a `context.Context` and error returns (`List` has none today), and `api.Server` should take an interface or a changed type; handlers map database failures to 5xx.
7. Connections: a pool bound derived from the numbers: 20 connections on the small plan and up to four instances during a deploy, so about 4 or 5 per instance at most.
8. Compliance: backup retention set to at most 30 days (the default 35 breaks the erasure rule); a way to erase all links of a closed account; data in `eu-west`.
9. Schema and migrations: a table for links with tags (an array column or a join table) and indexes for the owner's newest-first list and the tag filter; migrations that cope with several instances starting at once (a lock or a separate step).
10. Rollout: the provisioning ticket (about three days) and the dependency review (about a week) planned; the switch-over loses the links in memory once; tests (an in-memory implementation behind the interface, or a database container in CI).

Credit also, within the ten: `/healthz` kept independent of the database (a failover would otherwise get every replica restarted); a pure-Go driver because of `CGO_ENABLED=0`; status Proposed.

Deduct:

- A store other than the managed PostgreSQL (SQLite, bbolt, files, Redis, object storage, a self-run database): the score is at most 3.
- Each claim that contradicts the materials (a single instance, persistent volumes, a persistent Redis, a retention of 35 days kept): 1 to 2 points.
- Padding: generic pros and cons of databases, tutorials, sections not tied to linkbox. A long ADR is not a better one.

## Bands

- 9–10: PostgreSQL with nearly all of points 2 to 10 grounded in the materials, concise and in the requested format.
- 7–8: PostgreSQL, the replica diagnosis and SQLite's rejection right, and most of IDs, Store API, connections and retention.
- 5–6: PostgreSQL for mostly generic reasons ("durable, scalable, the team knows it") with a few grounded consequences; misses the replica diagnosis or the ID problem.
- 3–4: PostgreSQL with errors or contradictions of the materials, or another store argued carefully.
- 1–2: another store with weak reasoning, or an ADR that does not decide.
- 0: no ADR, or one that ignores the task.
