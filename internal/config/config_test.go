package config

import "testing"

// AT-B2: Config.Load + envOr — plan 03 §1 (all config from env, Vault-rendered).
func TestLoadReadsEnvAndDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("OIDC_ISSUER_URL", "https://kc/realms/r")
	t.Setenv("OIDC_AUDIENCE", "reckonna")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("PORT", "")                   // unset → default 8080
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "") // unset → default homelab

	c := Load("command")
	if c.ServiceName != "command" {
		t.Errorf("ServiceName = %q, want command", c.ServiceName)
	}
	if c.DatabaseURL != "postgres://x" {
		t.Errorf("DatabaseURL = %q", c.DatabaseURL)
	}
	if c.OIDCIssuer != "https://kc/realms/r" {
		t.Errorf("OIDCIssuer = %q", c.OIDCIssuer)
	}
	if c.OIDCAudience != "reckonna" {
		t.Errorf("OIDCAudience = %q", c.OIDCAudience)
	}
	if c.OTLPEndpoint != "http://otel:4318" {
		t.Errorf("OTLPEndpoint = %q", c.OTLPEndpoint)
	}
	if c.Port != "8080" {
		t.Errorf("Port default = %q, want 8080", c.Port)
	}
	if c.Environment != "homelab" {
		t.Errorf("Environment default = %q, want homelab", c.Environment)
	}
}

func TestLoadOverridesDefaults(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "prod")
	c := Load("query")
	if c.Port != "9090" || c.Environment != "prod" {
		t.Errorf("overrides not applied: Port=%q Env=%q", c.Port, c.Environment)
	}
}
