#!/usr/bin/env bash
# .claude/hooks/check-ledger-invariant.sh — backend-engineer Stop (-> scoped SubagentStop)
# exit 2 forces the head to keep working until every money path is tested.
set -uo pipefail
if ! grep -rqiE 'unbalanced|ErrUnbalanced|借方|debit.*credit' internal/ --include='*_test.go' 2>/dev/null; then
  echo "Money path untested: add an unbalanced-ledger rejection case before finishing." >&2
  exit 2
fi
if ! go test ./internal/... -run 'Ledger|Balance|Post' 2>&1; then
  echo "Ledger/balance test failing — fix before finishing." >&2
  exit 2
fi
exit 0
