---
description: Run the full loop across the 3 heads, then review.
---
Feature: $ARGUMENTS
Preconditions: plans/<feature>.md status: approved AND design/<feature>.design-system.html approved.
If ruflo MCP is available, init a hierarchical swarm and run the backend/frontend/infra heads in
parallel; else sequentially. Then /review.
  npx ruflo swarm init --topology hierarchical --max-agents 8 --strategy specialized
