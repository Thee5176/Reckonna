# Terraform — Kubernetes namespace for the Postgres workload. The actual
# StatefulSet/Service/Secret manifests live under infra/k8s/postgres and are
# applied with `kubectl apply -k` (or Argo) rather than via Terraform, to keep
# kustomize as the single source of declarative truth for app manifests.
#
# Terraform's job here is the cluster-level scaffolding (namespaces, RBAC
# bindings, CRDs) that does not belong in kustomize bases.

resource "kubernetes_namespace" "postgres" {
  metadata {
    name = "postgres"
    labels = {
      "app.kubernetes.io/name"                = "postgres"
      "app.kubernetes.io/part-of"             = "reckonna"
      "pod-security.kubernetes.io/enforce"    = "restricted"
      "pod-security.kubernetes.io/audit"      = "restricted"
      "pod-security.kubernetes.io/warn"       = "restricted"
      "kubernetes.io/metadata.name"           = "postgres"
    }
  }
}

output "postgres_namespace" {
  value       = kubernetes_namespace.postgres.metadata[0].name
  description = "Namespace that hosts the PG StatefulSet."
}
