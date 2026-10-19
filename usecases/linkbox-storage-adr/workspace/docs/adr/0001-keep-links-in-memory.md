# 1. Keep links in memory

## Status

Accepted, 2026-03-02.

## Context

linkbox is a prototype for a handful of colleagues. It runs as a single instance on the team's VM, and nobody
relies on it yet. We want to learn which features people use before we choose a database.

## Decision

Links live in a map inside the process (`internal/store`), guarded by a mutex. IDs come from a counter in the
store. Nothing is written to disk.

## Consequences

- No dependencies and nothing to operate: the service is one binary.
- Every restart loses every link. That is acceptable while linkbox is a prototype.
- Only one instance can run: a second one would hold links of its own.

## Alternatives considered

- SQLite on the VM's disk: durable and simple, but a dependency, and cgo or a pure-Go port, for data that nobody
  needs to keep yet.
- PostgreSQL on the VM: more to install and operate than a prototype needs.

Revisit when people rely on their links, or when linkbox needs more than one instance.
