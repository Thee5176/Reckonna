export const meta = {
  name: 'team-lead',
  description:
    'Lead orchestrates infra -> (backend + frontend): the infra contract flows downstream via return value. No subagent-to-subagent messaging.',
  whenToUse:
    'Run the 3 V-model heads where backend and frontend both depend on infra head outputs (endpoints, env schema, k8s names).',
  phases: [
    { title: 'Infra', detail: 'infra head emits the shared contract' },
    { title: 'Build', detail: 'backend + frontend consume the contract in parallel' },
  ],
}

// --- inputs (pass via Workflow args) ---------------------------------------
// args: { feature?: string, plan?: string, reconcile?: boolean }
const feature = args?.feature ?? 'the approved plan'
const planPath = args?.plan ?? 'plans/'
const reconcile = args?.reconcile ?? false

// --- the contract infra publishes for the other two heads ------------------
// This is the "memory" backend/frontend would otherwise ask infra for.
// secrets-vault rule: env schema carries NAMES + Vault paths, never values.
const CONTRACT_SCHEMA = {
  type: 'object',
  required: ['endpoints', 'envSchema', 'k8sNames'],
  properties: {
    endpoints: {
      type: 'array',
      description: 'service URLs the command/query services expose',
      items: {
        type: 'object',
        required: ['service', 'url', 'port'],
        properties: {
          service: { type: 'string' },
          url: { type: 'string' },
          port: { type: 'integer' },
        },
      },
    },
    envSchema: {
      type: 'array',
      description: 'env var NAME + its Vault path. NEVER a secret value.',
      items: {
        type: 'object',
        required: ['name', 'vaultPath'],
        properties: {
          name: { type: 'string' },
          vaultPath: { type: 'string' },
        },
      },
    },
    k8sNames: {
      type: 'array',
      description: 'k8s resource names (services, configmaps)',
      items: { type: 'string' },
    },
    notes: { type: 'string' },
  },
}

// --- PHASE 1: infra head publishes the contract ----------------------------
phase('Infra')
const contract = await agent(
  `You are the infra head for ${feature} (plan dir: ${planPath}).
   Produce the infra CONTRACT that the backend and frontend heads build against.
   Include:
   - endpoints: url + port for cmd/command and cmd/query
   - envSchema: each env var NAME + its Vault path ONLY. NEVER a secret value (secrets-vault rule).
   - k8sNames: service/configmap names
   Return ONLY the contract data — this is consumed by code, not a human.`,
  { label: 'infra:contract', phase: 'Infra', agentType: 'infra-engineer', schema: CONTRACT_SCHEMA }
)

// Verification gate: no contract -> do not start downstream build.
if (!contract) throw new Error('infra head produced no contract; aborting downstream build')
log(`infra contract ready: ${contract.endpoints.length} endpoints, ${contract.envSchema.length} env vars`)

const brief = JSON.stringify(contract, null, 2)

// --- PHASE 2: backend + frontend consume the contract (parallel) -----------
phase('Build')
const [backend, frontend] = await parallel([
  () =>
    agent(
      `You are the backend head for ${feature}.
       INFRA CONTRACT (read before coding; do NOT re-derive it):
       ${brief}
       Wire cmd/command + cmd/query to these endpoints and env schema.
       Enforce the ledger invariant. TDD: write the failing test first.`,
      { label: 'backend:build', phase: 'Build', agentType: 'backend-engineer' }
    ),
  () =>
    agent(
      `You are the frontend head for ${feature}.
       INFRA CONTRACT (read before coding):
       ${brief}
       Point the RN/Expo app at these endpoints. TDD: write the failing UI test first.`,
      { label: 'frontend:build', phase: 'Build', agentType: 'frontend-engineer' }
    ),
])

// --- OPTIONAL bidirectional round (args.reconcile) -------------------------
// If BE/FE surface needs infra must satisfy, re-spawn infra with those needs,
// then hand the revised contract back. Lead-as-bus; heads stay open-loop.
let revisedContract = null
if (reconcile && (backend || frontend)) {
  phase('Infra')
  revisedContract = await agent(
    `You are the infra head for ${feature}. Your original contract was:
     ${brief}
     The backend head reported:
     ${backend ?? '(no output)'}
     The frontend head reported:
     ${frontend ?? '(no output)'}
     Revise the contract to satisfy any unmet needs. Same schema, same rules (no secrets).`,
    { label: 'infra:reconcile', phase: 'Infra', agentType: 'infra-engineer', schema: CONTRACT_SCHEMA }
  )
}

return { contract: revisedContract ?? contract, backend, frontend }
