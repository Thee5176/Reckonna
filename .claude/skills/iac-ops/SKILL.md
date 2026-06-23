---
name: iac-ops
description: Vendor-neutral IaC + observability conventions. Use for Terraform/K8s/OTel/CI work.
---
- Terraform modules cloud-agnostic; secrets via the vault provider data sources (never .tfvars).
- K8s manifests vendor-neutral; validate with kubeconform -strict.
- OTel: OTLP exporter; every new endpoint/screen emits a span.
- GitHub Actions: go test -race + jest + e2e + terraform validate + Sonar + gitleaks. apply = human-gated.
