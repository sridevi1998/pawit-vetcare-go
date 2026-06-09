package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"pawit-vetcare/internal/config"
)

func TestValidateTokenAcceptsHS256Token(t *testing.T) {
	server := tokenTestServer()
	token := signedTestToken(t, "HS256", map[string]any{
		"sub":       "user_123",
		"role":      "ClinicAdmin",
		"tenant_id": "tenant_123",
		"exp":       time.Now().Add(time.Hour).Unix(),
	})

	auth, err := server.validateToken(token)
	if err != nil {
		t.Fatalf("expected token to validate: %v", err)
	}
	if auth.UserID != "user_123" || auth.Role != "ClinicAdmin" || auth.TenantID != "tenant_123" {
		t.Fatalf("unexpected auth context %#v", auth)
	}
}

func TestValidateTokenRejectsUnsupportedAlgorithm(t *testing.T) {
	server := tokenTestServer()
	token := signedTestToken(t, "none", map[string]any{
		"sub":       "user_123",
		"role":      "ClinicAdmin",
		"tenant_id": "tenant_123",
		"exp":       time.Now().Add(time.Hour).Unix(),
	})

	if _, err := server.validateToken(token); err == nil {
		t.Fatal("expected unsupported algorithm to be rejected")
	}
}

func TestValidateTokenRejectsMissingClaims(t *testing.T) {
	server := tokenTestServer()
	token := signedTestToken(t, "HS256", map[string]any{
		"sub": "user_123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	if _, err := server.validateToken(token); err == nil {
		t.Fatal("expected missing required claims to be rejected")
	}
}

func TestValidateTokenRejectsExpiredToken(t *testing.T) {
	server := tokenTestServer()
	token := signedTestToken(t, "HS256", map[string]any{
		"sub":       "user_123",
		"role":      "ClinicAdmin",
		"tenant_id": "tenant_123",
		"exp":       time.Now().Add(-time.Hour).Unix(),
	})

	if _, err := server.validateToken(token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func tokenTestServer() *Server {
	return &Server{cfg: config.Config{JWTSigningKey: "test-signing-key"}}
}

func signedTestToken(t *testing.T, algorithm string, claims map[string]any) string {
	t.Helper()

	header := map[string]any{"alg": algorithm, "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("encode token header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("encode token claims: %v", err)
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims

	mac := hmac.New(sha256.New, []byte("test-signing-key"))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature
}
