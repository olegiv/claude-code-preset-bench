# linkbox

A small HTTP service for saving links, written in Go with the standard library only.

Every request carries an API key in the `X-API-Key` header. Keys come from a file with one `key:user` pair
per line; each user sees and changes only their own links.

| Request | Result |
|---|---|
| `POST /links` with JSON `{"url", "title", "tags"}` | 201 and the new link |
| `GET /links?tag=` | 200 and the caller's links, newest first, optionally only those with a tag |
| `GET /links/{id}` | 200 and the link, or 404 |
| `DELETE /links/{id}` | 204, or 404 |
| `GET /healthz` | 200, without a key |

```bash
go run ./cmd/linkbox -addr 127.0.0.1:8080 -keys keys.txt
go test ./...
```
