package config

import "os"

// Config is the runtime configuration for a service. Every value is read from
// the process environment, which is Vault-rendered at runtime (vault agent /
// k8s Vault Agent Injector) — never committed. See .claude/rules/secrets-vault.md.
type Config struct {
	ServiceName  string // "command" | "query" (span/resource attribute)
	DatabaseURL  string // DATABASE_URL
	OIDCIssuer   string // OIDC_ISSUER_URL (Keycloak discovery root)
	OIDCAudience string // OIDC_AUDIENCE
	Port         string // PORT (default 8080)
	OTLPEndpoint string // OTEL_EXPORTER_OTLP_ENDPOINT (empty → telemetry not exported)
	Environment  string // DEPLOYMENT_ENVIRONMENT (default homelab) → deployment.environment resource attr
}

// Load reads configuration from the environment for the named service.
func Load(serviceName string) Config {
	return Config{
		ServiceName:  serviceName,
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		OIDCIssuer:   os.Getenv("OIDC_ISSUER_URL"),
		OIDCAudience: os.Getenv("OIDC_AUDIENCE"),
		Port:         envOr("PORT", "8080"),
		OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		Environment:  envOr("DEPLOYMENT_ENVIRONMENT", "homelab"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
