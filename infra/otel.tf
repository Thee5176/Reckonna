# Terraform — Kubernetes namespace for the OpenTelemetry Collector DaemonSet.
# Same single-owner contract as postgres + redis: kustomize does NOT declare
# this Namespace. App manifests under infra/k8s/otel/ are applied with
# `kubectl apply -k` (human-only).

resource "kubernetes_namespace" "otel" {
  metadata {
    name = "otel"
    labels = {
      "app.kubernetes.io/name"    = "otel-collector"
      "app.kubernetes.io/part-of" = "reckonna"
      # PSA exception: the OTel Collector DaemonSet uses hostNetwork: true +
      # hostPort 4317 (gRPC) + hostPort 4318 (HTTP) so workloads reach the
      # node-local receiver via $(NODE_IP) from the downward API. PSA
      # `restricted` and `baseline` both forbid hostNetwork / hostPort, so
      # this single namespace must run `privileged`. The container
      # securityContext stays hardened (runAsNonRoot=10001, drop ALL caps,
      # readOnlyRootFilesystem=true, no privilege escalation) — the
      # relaxation is only at the namespace admission layer, not the
      # workload.
      "pod-security.kubernetes.io/enforce" = "privileged"
      "pod-security.kubernetes.io/audit"   = "privileged"
      "pod-security.kubernetes.io/warn"    = "privileged"
      "kubernetes.io/metadata.name"        = "otel"
    }
  }
}

output "otel_namespace" {
  value       = kubernetes_namespace.otel.metadata[0].name
  description = "Namespace that hosts the OTel Collector DaemonSet."
}
