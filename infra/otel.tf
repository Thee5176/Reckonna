# Terraform — Kubernetes namespace for the OpenTelemetry Collector DaemonSet.
# Same single-owner contract as postgres + redis: kustomize does NOT declare
# this Namespace. App manifests under infra/k8s/otel/ are applied with
# `kubectl apply -k` (human-only).

resource "kubernetes_namespace" "otel" {
  metadata {
    name = "otel"
    labels = {
      "app.kubernetes.io/name"             = "otel-collector"
      "app.kubernetes.io/part-of"          = "reckonna"
      "pod-security.kubernetes.io/enforce" = "restricted"
      "pod-security.kubernetes.io/audit"   = "restricted"
      "pod-security.kubernetes.io/warn"    = "restricted"
      "kubernetes.io/metadata.name"        = "otel"
    }
  }
}

output "otel_namespace" {
  value       = kubernetes_namespace.otel.metadata[0].name
  description = "Namespace that hosts the OTel Collector DaemonSet."
}
