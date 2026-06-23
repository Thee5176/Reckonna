# Accounting CQRS — Go/Gin rewrite (Claude Code working directory)

[![Quality Gate Status](https://sonar.thee5176.com/api/project_badges/measure?project=reckonna&metric=alert_status&token=sqb_25053392f3db17cc5bd4d723139d0bc9492bf8ed)](https://sonar.thee5176.com/dashboard?id=reckonna)
[![Technical Debt](https://sonar.thee5176.com/api/project_badges/measure?project=reckonna&metric=software_quality_maintainability_remediation_effort&token=sqb_25053392f3db17cc5bd4d723139d0bc9492bf8ed)](https://sonar.thee5176.com/dashboard?id=reckonna)
[![Security Issues](https://sonar.thee5176.com/api/project_badges/measure?project=reckonna&metric=software_quality_security_issues&token=sqb_25053392f3db17cc5bd4d723139d0bc9492bf8ed)](https://sonar.thee5176.com/dashboard?id=reckonna)

V-model applied INTO an agent swarm. Source of truth: Confluence
"Claude Code Setup — V-Model × Swarm (3 head agents · V-phase skills · plugins core)".

## Architecture (two layers)
- HEAD agents (opus) — 3: backend / frontend / infra-engineer. Each owns its domain's full V.
- V-phase SKILLS (the role procedures the heads invoke, on sonnet via ruflo):
  domain-modeler (design) → tdd-implementer / migration-mapper (build) → code-reviewer (verify).

## Build order (run in this sequence)
1. Bootstrap: `export CLAUDE_BOOTSTRAP=1`, then scaffold the Go monorepo (go.mod, dirs).
   The bypass lets you edit internal/ cmd/ db/ before any plan exists.
2. Keep the `.claude/` artifacts (present). Install plugins (below). Then `unset CLAUDE_BOOTSTRAP`.
3. Run the loop: `/plan` (approve Step 1 spec + Step 2 design) → `/ship` → `/review`.
   Migration: `/migrate-endpoint <java-path>` ; single unit: `/tdd <unit>`.

## Plugins (core; each self-wires its hooks)
    claude plugin marketplace add obra/superpowers-marketplace
    claude plugin install superpowers@superpowers-marketplace
    npx ruflo init                 # FULL install: MCP server (needed for the swarm)
    uv tool install graphifyy && graphify install --project && graphify .
    claude plugin marketplace add JuliusBrussee/caveman
    claude plugin install caveman@caveman

## Gates (our hooks; in settings.json)
- UserPromptSubmit: require-plan.sh (allowlists /plan, /tdd, /migrate-endpoint, scaffold)
- PreToolUse(Edit|Write): require-prereq.sh (spec/design gate), no-secrets.sh
- PostToolUse(Edit|Write): verify-go/frontend/infra.sh
- SubagentStop: sonar-quality-gate.sh ; backend-engineer Stop → check-ledger-invariant.sh

## Still verify (page §10 warning)
- frontend-design skill availability in your Claude Code (/plan Step 2 engine)
- ruflo config keys: disable SPARC autoplan; set worker tier = sonnet
- ruflo MCP present at runtime (else heads run skills sequentially on opus — no cost split)
- SonarQube per-SubagentStop cost with a wide swarm
