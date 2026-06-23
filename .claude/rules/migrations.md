---
paths: ["db/migration/**"]
---
# Migration rules (load only when touching db/migration/**)
- Every up migration has a matching down. UUIDv7 PKs.
- Enforce 借方=貸方 with CHECK constraints at the DB layer, not just in Go.
- Never edit an applied migration; add a new one.
