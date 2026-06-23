#!/usr/bin/env bash
# .claude/hooks/verify-go.sh — PostToolUse(Edit|Write): gofmt + build + vet + lint + test for *.go
set -uo pipefail
INPUT=$(cat); FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
case "$FILE" in *.go) ;; *) exit 0 ;; esac
gofmt -w "$FILE"
go build ./...        2>&1 || { echo "go build failed" >&2; exit 2; }
go vet ./...          2>&1 || { echo "go vet failed"   >&2; exit 2; }
golangci-lint run     2>&1 || { echo "golangci-lint failed" >&2; exit 2; }
go test ./... -race   2>&1 || { echo "go test failed" >&2; exit 2; }
exit 0
