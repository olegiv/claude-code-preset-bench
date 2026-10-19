# 2. Store links in the platform's managed PostgreSQL

## Status

Proposed, 2026-10-19.

## Context

ADR 0001 kept links in memory because linkbox was a prototype running as a single instance on the team's VM. Both
premises are gone:

- People rely on their links now, and every deploy wipes them (ticket #4127; we deploy two or three times a week,
  `ops/deployment.md`).
- linkbox has run on the container platform since 2026-07-14, where policy requires at least two replicas behind a
  round-robin load balancer without session affinity, and up to four instances run during a deploy
  (`ops/deployment.md`). Each instance holds its own map, so users see different links from request to request.
  That explains the other tickets: a link that disappears and comes back (#4188), different lists on two devices
  (#4260), deleted links that return (#4302). Each instance also numbers links with its own counter
  (`store.go`: `fmt.Sprintf("l%d", s.seq)`), so `l7` names a different link on each replica (#4201).

So we need one store shared by every instance, which survives restarts and deploys. Constraints:

- Container filesystems are ephemeral and the platform offers no persistent volumes; state belongs in a platform
  data service, and teams may not run databases of their own (`ops/deployment.md`, `ops/platform-services.md`).
- Links are personal data: EU region only, encrypted at rest, backups that can undo an operator's mistake, and
  erasure within 30 days of an account closure, backups included (`compliance.md`).
- Load is small: 4 to 6 requests per second at peak, 95% reads, about 3,500 new links a day, users tripling within
  a year; target p99 under 100 ms (`metrics.md`).
- Builds run with `CGO_ENABLED=0`, and a third-party module needs a security review of about a week (`team.md`).

## Decision

Store links in the platform's managed PostgreSQL 16, small plan, in `eu-west`.

- Schema: `links(id, owner, url, title, created_at)` with tags as a `text[]` column and a GIN index for the tag
  filter, plus an index on `(owner, created_at DESC, id DESC)` for the newest-first list.
- IDs come from the database (an identity column, exposed as `l<n>` to keep the API's format), not from a counter
  in the process: a counter restarts at `l1` on every start and collides between instances. The newest-first tie
  break (`compareIDs`) becomes `ORDER BY created_at DESC, id DESC`.
- `internal/store` gets a PostgreSQL implementation behind a small interface that `api.Server` takes instead of
  `*store.Store`. Its methods take a `context.Context` and return errors, `List` included (today it cannot fail);
  the handlers answer 503 when the database is unavailable. The in-memory implementation stays for unit tests.
- Driver: `pgx` through `database/sql`, pure Go, so `CGO_ENABLED=0` builds keep working.
- Connections: the small plan allows 20 and up to four instances run during a deploy, so each instance's pool is
  capped at 4 (`SetMaxOpenConns(4)`), which leaves 4 for migrations and operators.
- Migrations: SQL files embedded in the binary and applied at startup under a PostgreSQL advisory lock, because
  several instances start at once.
- Backup retention: set to 28 days instead of the default 35, so that erased data leaves the backups within the
  30 days the policy allows. Account closures are handled with a `DELETE FROM links WHERE owner = $1`, run from a
  small admin command when the ticket arrives.

## Consequences

- Links survive deploys, and every instance sees the same links: the five tickets' causes go away.
- `/healthz` stays a liveness check that does not query the database; otherwise a database failover (about 30
  seconds) would make the platform restart every replica.
- New work: the store interface and its PostgreSQL implementation, `context` through the handlers, migrations, the
  admin erase command, and CI tests against a PostgreSQL service container.
- Lead time: the database ticket takes about three working days and `pgx`'s security review about a week; both
  start now.
- The switch-over loses the links held in memory once, as every deploy does today; we announce it.
- Cost and operations: one small database; the platform team operates it.

## Alternatives considered

- SQLite (the brief's suggestion) or bbolt: a file per instance. With two to four instances each would keep its own
  links, exactly today's bug, and the file disappears with the container, since there are no persistent volumes.
  Replication tools such as Litestream or LiteFS allow a single writer and still need a volume.
- Managed Redis: a cache tier without persistence or backups; evictions would lose links.
- Object storage: no conditional writes, so instances updating the same object lose each other's changes, and at
  20 to 80 ms a request, listing a user's links object by object misses the 100 ms target.
- Running our own database on the platform: not allowed, and it would have no persistent volume.
- Keeping links in memory with sticky sessions: the load balancer has no session affinity, and deploys would still
  wipe the links.
