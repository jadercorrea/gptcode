# Enforce cache expiration

`Cache.Get` currently returns entries after their TTL has elapsed, and `Len`
continues to count those stale entries.

Make expiration authoritative while preserving the exported API. A lookup at
the exact expiration instant is expired and must remove the stale entry.
Maintain race-free concurrent reads and writes; do not introduce background
goroutines or wall-clock sleeps.

The implementation must pass:

```text
go test -race ./...
go vet ./...
```
