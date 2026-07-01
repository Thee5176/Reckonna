# Dev Orchestration Plan — Plans 01 / 02 / 03 (DRAFT)

status: draft (rough idea — not approved, not executing)
author: thee5176 (via Claude lead)
created: 2026-06-24
scope: how the three approved/pending plans get built in parallel — phasing, agent
       roster, per-subagent token budget, and progress tracking.

> This is an **orchestration** doc, not a feature plan. The feature plans
> (`plans/01-*`, `plans/02-*`, `plans/03-*`) remain the source of truth for *what* to
> build. This doc describes *who* builds it, *in what order*, *at what cost*, and *how we
> track it*. Nothing here overrides `devops.md` gates: status flips and `terraform/kubectl
> apply` stay human-only.

---

## Preconditions (must clear before Phase 1)

- Plan 01 `infra-postgres-tailnet` — **approved** ✅
- Plan 02 `infra-k8s-cloudflare-tunnel` — `approve-with-edits` (eng-review 2026-06-23); P1 edits + human `status: draft → approved` pending.
- Plan 03 `backend-cqrs-core` — `approve-with-edits` (eng-review 2026-06-23); P1 edits + human flip pending.

Parallel-dev is technically valid (verified): plan 03 reaches full green in isolation via
`testcontainers-go` (Postgres) + mock-JWKS (Keycloak); plans 01/02 emit only
manifests/TF/scripts (no apply). Dependencies are **deploy/runtime-only**, not build-time.
The only hard serialization is the human deploy gate at the end: 01 → 02 → 03.

---

## Phasing (gated)

```
PHASE 0  Unblock          2 agents · apply P1 edits → human flips status
   │
   ▼ (human approval gate — devops.md require-prereq)
PHASE 1  Parallel build              two independent tracks, different HEADs
   ├─ Track A  Backend plan 03   (backend-engineer HEAD + sonnet swarm)
   └─ Track B  Infra 01 → 02     (infra-engineer HEAD + sonnet swarm)
   │
   ▼ (CI green + code-reviewer gate per cluster)
PHASE 2  Converge          deploy gate (human-only apply): 01 → 02 → 03
```

Track A and B run concurrently (independent: testcontainers + mock-JWKS). Within each
track, steps chain per TDD (Red → Green → Refactor).

---

## Agent roster + responsibility

| Agent | Model | Owns | V-phase skills invoked |
|-------|-------|------|------------------------|
| **backend-engineer** (HEAD) | Opus | Plan 03 whole V — writes AT/IT RED first, orchestrates, enforces 借方=貸方 | domain-modeler → tdd-implementer → code-reviewer |
| **infra-engineer** (HEAD) | Opus | Plans 01+02 V — manifests/TF/scripts, no apply | iac-ops → code-reviewer |
| **lead** | Opus | Cross-track orchestration, memory-bus verify, human reporting | — |

HEADs dispatch short-lived **sonnet workers** per cluster (tier-3 routing per CLAUDE.md).
Workers are one-shot, memory-as-bus, degraded-mode briefed. No subagent↔subagent comms
(per CLAUDE.md Agent-Comms rules).

### Track A — Plan 03 clusters (dependency-ordered)

| Cluster | Steps | Worker | Depends on | Parallel? |
|---------|-------|--------|-----------|-----------|
| A1 domain | S1→S2 | `be-domain` | 00 | ✅ start immediately |
| A2 db+migrations | S3→S4, S5, S5a→S5b→S5c, S9a | `be-db` | 00 | ✅ parallel w/ A1 |
| A3 command path | S6→S6a→S7→S8→S8a→S8b→S9 | `be-command` | A1, A2 | after A1+A2 |
| A4 auth | S10 | `be-auth` | 00, kc-mock | ✅ parallel w/ A1/A2 |
| A5 query path | S11→S12→S13→S14 | `be-query` | A2, A4 | after A2+A4 |
| A6 wire+obs+e2e | S15→S16→S17(split)→S17b | `be-wire` | A3, A5 | last |

### Track B — Plans 01 + 02

| Cluster | Steps | Worker | Depends on |
|---------|-------|--------|-----------|
| B1 plan01 postgres | 01:S1–S6 (StatefulSet, Vault injector, svc, tailscale vals, pg-endpoint.sh) | `infra-pg` | 00 (approved) |
| B2 plan02 tunnel | 02:S0–S6 (CF TF, cloudflared, nginx ConfigMap, HA, secrets, docs) | `infra-tunnel` | 00, B1 *pattern* |

---

## Per-subagent cost (token budget)

Anchored on observed data: graphify extraction worker = ~109k tokens (8 files + JSON);
eng-review worker ≈ 130k (7 files + 30KB report). Coding+TDD iterate higher (re-reads
context, runs tests 2–3×).

| Agent | Model | Est. tokens | Basis | ~Cost* |
|-------|-------|------------:|-------|-------:|
| Phase 0 ×2 (edit 02/03) | Opus | 180k | review ≈130k, edits lighter | ~$4 |
| backend-engineer HEAD | Opus | 600k | orchestration, RED specs, verify loop | ~$18 |
| `be-domain` A1 | Sonnet | 180k | 2 steps, table-driven money tests | ~$1.4 |
| `be-db` A2 | Sonnet | 320k | 8 migrations + seed + sqlc gen | ~$2.5 |
| `be-command` A3 | Sonnet | 380k | 7 steps, handler+mw+idempotency | ~$3.0 |
| `be-auth` A4 | Sonnet | 200k | JWKS mw + mock-JWKS tests | ~$1.6 |
| `be-query` A5 | Sonnet | 300k | 4 read endpoints + statements | ~$2.4 |
| `be-wire` A6 | Sonnet | 350k | router wire + OTel + split e2e | ~$2.8 |
| code-reviewer ×6 | Sonnet | 720k | diff review, ~130k observed | ~$5.7 |
| infra-engineer HEAD | Opus | 450k | 2 plans orchestration | ~$13 |
| `infra-pg` B1 | Sonnet | 260k | manifests + script + fake-shim tests | ~$2.0 |
| `infra-tunnel` B2 | Sonnet | 280k | TF + manifests + HA + docs | ~$2.2 |
| map/`graphify --update` ×4 | Sonnet | 320k | AST-only re-index cheap | ~$2.5 |
| **TOTAL** | | **≈ 4.7M** | | **≈ $63** |

*Cost basis: Opus blended ~$30/M; Sonnet blended ~$8/M. Rough — varies with iteration
count + prompt-cache hits. HEAD prompt-cache (5min TTL) cuts repeat reads; batching a
cluster's steps into one worker beats one-agent-per-step (shared context stays cached).

---

## Progress tracking — 5 layers

| Layer | Mechanism | Source-of-truth for |
|-------|-----------|--------------------|
| 1. Live board | harness TaskCreate/TaskUpdate (one task per cluster) | what's running now; what lead polls |
| 2. Git | `Plan: S<n>` commit trailer, one-step-one-commit (devops.md) | landed steps — authoritative |
| 3. Impl tracker | `plans/0X-*.impl.md` per plan (like `00-*.impl.md`) | audit trail / human review |
| 4. Memory-bus | `memory_store` keys `phaseN/<agent>/<output>` | inter-agent handoff (degraded-safe) |
| 5. CI + graph | `go test -race` + Sonar + gitleaks green = merge gate; `graphify --update` every 5 files | "done" definition + drift detection |

**Per-cluster loop:** lead spawns worker → worker writes outputs + memory key → lead
verifies key + runs tests → spawns code-reviewer → green → commit with `Plan: S<n>` →
update impl tracker + TaskUpdate completed → next cluster. No polling — lead waits for
completion notifications.

Sample board, Phase-1 mid-flight:

```
[A1 domain]      completed     S1,S2 @ <sha>
[A2 db]          in_progress   S3,S4 done · S5 sqlc-gen running
[A3 command]     blocked       waits A1+A2
[A4 auth]        in_progress   S10 mock-JWKS RED
[B1 pg]          completed     01:S1–S6 @ <sha>
[B2 tunnel]      in_progress   02:S2 cloudflared
```

---

## Open decisions (human)

- P1 edits to plans 02 + 03 — apply before or after status flip? (recommend: before)
- Taste-flags carried from eng-review:
  - Plan 02: OTel waiver for nginx harness (explicit exception vs document); token-rotation test-vs-document.
  - Plan 03: migration-numbering authority (avoid two branches grabbing `005`); OpenAPI drift-window bound.
- Cost dial: Sonnet-only (halve cost) vs current Opus-HEAD mix; split backend into more parallel workers vs batched clusters.

## Deferred / out of scope

- Frontend (RN/Expo) — later feature plan.
- Runtime Vault rendering, real Keycloak provisioning (`infra/keycloak-oidc`) — separate infra tasks; block plan 03 at *runtime* only.
- The actual `apply` (terraform/kubectl/helm) — human-only, Phase 2.
