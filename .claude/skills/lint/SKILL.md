---
name: lint
description: Run the installed linters, resolve each linting point one by one, and re-validate until clean. Use when asked to lint, fix lint warnings, or clear linter output.
---
# Lint

Use this skill when the user wants lint output cleaned up rather than explained.

## Workflow
1. Identify the relevant lint commands for the touched files or repo surface.
2. Run the installed linters and capture every warning or error.
3. Fix the first concrete lint point in the smallest possible edit.
4. Re-run the narrowest affected linter immediately after that edit.
5. Repeat until the lint output is clean or the next failure requires user input.

## Rules
- Treat each lint finding as a separate item to resolve.
- Prefer the smallest behavior-preserving edit that satisfies the linter.
- Do not widen scope until the current lint point is cleared.
- If a tool is missing, report it clearly and use the closest available validator.
- Keep code and documentation style consistent with the repository.

## Typical checks
- Shell scripts: `shellcheck`
- GitHub Actions: `actionlint`
- Markdown: `markdownlint`
- Terraform: `terraform fmt -check` and `terraform validate`
- Go: repository-specific lint or test commands already used by the project

## Completion
Return only when each reported lint point is either fixed or explicitly blocked by a missing tool or external constraint.