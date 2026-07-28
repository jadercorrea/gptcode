# Contain writes inside the store root

`Store.Write` accepts attacker-controlled names. Traversal, absolute paths, and
symlinked parent directories can currently write outside the configured root.

Contain every write beneath the root while preserving the exported API and
support for legitimate nested paths. Reject unsafe input before creating
directories or files. Do not resolve the problem by deleting symlinks, changing
the process working directory, or weakening file permissions.

The implementation must pass:

```text
go test -race ./...
go vet ./...
```
