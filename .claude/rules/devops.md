---
description: DevOps & delivery policy (loads every session, all agents)
---
# DevOps — delivery rules
- Work only from the approved plans/<feature>.md (require-plan + require-prereq gates).
- Backend needs the approved spec; frontend needs the approved design system, FIRST.
- One step = one commit, in plan order, with a `Plan: S<n>` trailer.
- Conventional Commits only. No force-push to shared branches. No squashing across steps.
- `terraform apply` and `kubectl delete` are human-only.
- CI (go test -race + jest + e2e + terraform validate + Sonar + gitleaks) green before merge.
- Observability is part of "done": new endpoints/screens emit OpenTelemetry spans.
