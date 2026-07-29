# Eliminate opposing-transfer deadlocks

Two goroutines transferring funds in opposite directions can each hold one
account lock while waiting forever for the other. A transfer to the same
account deterministically attempts to lock the same mutex twice.

Fix both deadlocks while preserving the existing exported API, error semantics,
and total balance. Keep the implementation idiomatic, `gofmt`-formatted, and
do not use package `unsafe`. The provided regression and quality tests must
pass:

```text
go test -race ./...
go vet ./...
```

Do not weaken or remove synchronization.
