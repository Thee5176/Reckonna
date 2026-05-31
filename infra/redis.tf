# Terraform — Kubernetes namespace for the Redis cache workload. Mirrors the
# postgres.tf single-owner contract: kustomize (infra/k8s/redis/) deliberately
# does NOT declare this Namespace — Terraform is the only writer. App manifests
# under infra/k8s/redis/ are applied with `kubectl apply -k` (human-only).

resource "kubernetes_namespace" "redis" {
  metadata {
    name = "redis"
    labels = {
      "app.kubernetes.io/name"             = "redis"
      "app.kubernetes.io/part-of"          = "reckonna"
      "pod-security.kubernetes.io/enforce" = "restricted"
      "pod-security.kubernetes.io/audit"   = "restricted"
      "pod-security.kubernetes.io/warn"    = "restricted"
      "kubernetes.io/metadata.name"        = "redis"
    }
  }
}

output "redis_namespace" {
  value       = kubernetes_namespace.redis.metadata[0].name
  description = "Namespace that hosts the Redis StatefulSet."
}
