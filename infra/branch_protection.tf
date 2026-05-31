# Branch protection on Thee5176/Reckonna `main` — IaC source of truth for
# the CI gate. Mirrors what's currently set via the GitHub UI; brings the
# rule under Terraform so future changes go through PR review.
#
# IMPORTANT: `terraform apply` is human-only (devops.md). The first apply
# must import the existing protection rule, otherwise the provider will
# fail with "Branch protection rule already exists":
#
#   terraform -chdir=infra import github_branch_protection.main \
#     'Thee5176/Reckonna:main'
#
# After the import, `terraform plan` should show no changes (or only the
# `SonarQube` context being added — see G8 wiring in .github/workflows/ci.yml).

resource "github_branch_protection" "main" {
  repository_id = "Reckonna"
  pattern       = "main"

  # All 6 CI jobs that must be green before a merge to main.
  # Names match `jobs.<id>.name` in .github/workflows/ci.yml exactly.
  required_status_checks {
    strict = true
    contexts = [
      "Go build · test -race · lint",
      "Jest",
      "E2E (testcontainers)",
      "Terraform validate",
      "gitleaks (secret scan)",
      "SonarQube",
    ]
  }

  required_pull_request_reviews {
    dismiss_stale_reviews           = true
    require_code_owner_reviews      = false
    required_approving_review_count = 0 # solo maintainer; raise to 1 if collaborators join
  }

  enforce_admins          = false # solo maintainer; enable when collaborators join
  required_linear_history = true
  allows_force_pushes     = false
  allows_deletions        = false
}
