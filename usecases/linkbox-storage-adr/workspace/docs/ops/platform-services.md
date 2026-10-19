# Platform data services

What the platform team runs for application teams; ask in #platform for anything not listed. Teams do not run
databases or other stateful software of their own on the platform.

## Managed PostgreSQL

- PostgreSQL 16: a primary with a standby in another zone; a failover takes about 30 seconds.
- Plans: small (2 vCPU, 4 GB memory, 50 GB disk, at most 20 client connections) and medium (4 vCPU, 16 GB memory,
  200 GB disk, at most 100 client connections).
- Encrypted at rest. Daily snapshots and point-in-time recovery; backup retention can be set from 7 to 35 days and
  is 35 days unless the team changes it.
- Available in `eu-west` and `us-east`. Provisioned through a platform ticket, which takes about three working
  days; the credentials arrive as a mounted secret.

## Managed Redis

- Redis 7 as a cache tier: no persistence, keys may be evicted under memory pressure, and there are no backups.
- Available in `eu-west` and `us-east`.

## Object storage

- S3-compatible buckets, encrypted at rest, with optional versioning. A read after a write sees the write.
- Each request takes 20 to 80 ms.
- No conditional writes: a PUT cannot be made to depend on the object's current version (no `If-Match` or
  `If-None-Match`). Listing is paginated at 1,000 keys per page.
- Available in every region.
