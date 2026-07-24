package config

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDCConfig is the resource-server side of Keycloak OIDC. Both values are
// supplied from Vault-rendered env at runtime (never hardcoded): the issuer URL
// (discovery root) and the expected audience.
type OIDCConfig struct {
	IssuerURL string
	Audience  string
}

// Authenticator validates RS256 access tokens against the provider's JWKS,
// checking signature, issuer, audience, and expiry, and extracts the sub claim.
type Authenticator struct {
	verifier *oidc.IDTokenVerifier
}

// NewAuthenticator fetches the provider's discovery document + JWKS and builds a
// verifier bound to the expected audience.
func NewAuthenticator(ctx context.Context, cfg OIDCConfig) (*Authenticator, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: cfg.Audience})
	return &Authenticator{verifier: verifier}, nil
}

// Verify validates a raw bearer token and returns its sub (owner id). Any
// failure — bad signature, wrong issuer/audience, expiry, missing sub — is an
// error, which the auth middleware maps to 401.
func (a *Authenticator) Verify(ctx context.Context, rawToken string) (string, error) {
	tok, err := a.verifier.Verify(ctx, rawToken)
	if err != nil {
		return "", fmt.Errorf("verify token: %w", err)
	}
	if tok.Subject == "" {
		return "", fmt.Errorf("token has no sub claim")
	}
	return tok.Subject, nil
}
