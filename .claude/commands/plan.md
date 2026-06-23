---
description: Two-step planner. Step 1 = backend spec (acceptance + integration). Step 2 = single-file HTML design system. Both human-approved gates.
---
Plan the feature in TWO steps, pausing for human approval after each. Feature: $ARGUMENTS

# STEP 1 — Backend spec (prerequisite gate for /backend)
Act as the planner (plan-writer + superpowers). ASK the developer for anything missing, then
write plans/<feature>.md:
  - Section 1 Acceptance-test spec: Given/When/Then per criterion (incl. invalid 借方≠貸方).
  - Section 2 Integration-test spec: CQRS write->read, tx atomicity, API request/response shapes.
  - Section 3 Implementation steps: files + verbatim commit messages (+ Plan: S<n>).
Set front-matter `status: draft`; on human approval flip to `status: approved`.
==> /backend is BLOCKED until Sections 1-2 exist AND status: approved.

# STEP 2 — Design system (prerequisite gate for /frontend)
Using the frontend-design skill, produce ONE self-contained HTML file:
  design/<feature>.design-system.html
Inline CSS only: design tokens, component gallery (buttons, inputs, JournalEntryForm, list,
loading/empty/error states), and the feature's screens. Developer reviews IN ADVANCE; on
approval add an `approved` marker.
==> /frontend is BLOCKED until design/<feature>.design-system.html exists AND is approved.
