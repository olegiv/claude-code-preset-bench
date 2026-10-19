# Brief: durable storage for linkbox

From Dana, team lead, 2026-10-16.

linkbox keeps every link in memory, so a deploy or a restart wipes everything, and support has started to forward
complaints (`support-tickets.md`). Product wants links to survive from this quarter on.

Please write ADR 0002: how linkbox stores links from now on. Use the format of ADR 0001
(`adr/0001-keep-links-in-memory.md`) and mark it Proposed; we review it together on Thursday. Keep it to what we
need to decide and to do.

My gut says SQLite: one file next to the binary, no new infrastructure to ask for, and our load is tiny
(`metrics.md`). Everything else you need should be in this folder and in the code.
