# Verified session store

This is the public fixture used by GPTCode's terminal demonstration. It keeps
the example small enough to audit while preserving production-grade properties:

- a documented expiration boundary;
- a concurrency-safe zero value;
- table-driven behavior tests;
- concurrent read/write verification with Go's race detector;
- 100% statement coverage enforced in CI.

Run the same verification used by the repository:

```bash
scripts/verify-public-examples.sh
```
