# Workflows

Deterministic multi-agent orchestration scripts run via the `Workflow` tool.
A workflow is plain JS: the script decides what fans out, what runs in
sequence, and what verifies — the orchestrator (not an LLM) holds state
between agents.

> Workflows spawn real agents and spend tokens. Only run one when the work
> genuinely needs multi-agent orchestration.

---

## `team-lead.js`

Lead orchestrates the three V-model heads where **backend and frontend both
depend on infra**. Infra runs first and publishes a contract; backend and
frontend consume it in parallel.

### Why it exists (the coordination problem)

Subagents are stateless one-shot workers: **subagent↔subagent messaging does
not exist** (see root `CLAUDE.md` → "Agent Comms"). Backend/frontend cannot
"ask" infra anything — infra has no inbox.

Two ways around it:

| Pattern | Bus | Used here |
|---------|-----|-----------|
| Manual `Agent` spawns | shared memory keys (`memory_store`/`memory_search`) + polling | no |
| `Workflow` script | the orchestrator's JS return value | **yes** |

Inside a workflow the lead is plain JS, so `const contract = await agent(...)`
holds infra's output in a variable and injects it into the downstream prompts.
No memory keys, no polling, no race. The return value **is** the bus.

### Flow

```
phase: Infra          infra-engineer  -> validated CONTRACT (schema-checked)
   │                     (endpoints, envSchema, k8sNames)
   ▼  verification gate: no contract -> throw, do not build
phase: Build          backend-engineer  ┐ parallel, both read the contract
                      frontend-engineer ┘
   ▼  (optional, args.reconcile)
phase: Infra          infra-engineer re-reads BE/FE output -> revised contract
```

- **Infra is first, not "asked"** — it's an upstream dependency, expressed as a
  sequential phase. Backend/frontend never talk to it; they read what it left.
- **Backend ∥ frontend** — independent of each other, so `parallel()`.
- **Reconcile round** is lead-as-bus: infra never receives a message; the
  orchestrator hands it the prior BE/FE outputs as prompt text.

### Run it

```js
// one-way (default): infra contract -> backend + frontend
Workflow({ name: "team-lead", args: {
  feature: "plan 02 reckonna-app",
  plan:    "plans/02-infra-k8s-cloudflare-tunnel.md"
}})

// bidirectional: add a reconcile round after the build
Workflow({ name: "team-lead", args: { feature: "...", plan: "...", reconcile: true }})
```

Invoking **runs** it — it dispatches the real heads doing real work. Give it a
concrete `feature` + `plan`; the defaults (`"the approved plan"`, `"plans/"`)
are placeholders.

### Args

| Key | Type | Default | Meaning |
|-----|------|---------|---------|
| `feature` | string | `"the approved plan"` | what the heads are building |
| `plan` | string | `"plans/"` | path to the approved plan dir/file |
| `reconcile` | boolean | `false` | add the infra↔build reconcile round |

### The contract (what infra publishes)

Schema-validated, so it cannot be malformed prose:

- `endpoints[]` — `{ service, url, port }` for `cmd/command` + `cmd/query`
- `envSchema[]` — `{ name, vaultPath }`. **Names + Vault paths only, never a
  secret value** (see `.claude/rules/secrets-vault.md`).
- `k8sNames[]` — service / configmap names
- `notes?` — free text

### Returns

```js
{ contract, backend, frontend }   // contract = revised one if reconcile ran
```

### When to use / not

- **Use** when backend + frontend can't start until infra settles its contract.
- **Don't** for a single head's work — call that head directly with the `Agent`
  tool. Don't add agents the dependency graph doesn't require.

### Anti-patterns (will silently fail)

- Telling backend/frontend to "ask infra" or "wait for infra" — no inbox, no
  wait mechanism; the subagent aborts or runs on stale assumptions.
- Spawning all three heads at once expecting them to chain via messages.

### Extending

Add a new workflow as `.claude/workflows/<name>.js` starting with the
`export const meta = {...}` block (pure literal: `name`, `description`,
`phases`), then document it here.
