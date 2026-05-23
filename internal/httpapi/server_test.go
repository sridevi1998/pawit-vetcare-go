package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"pawit-vetcare/internal/config"
	"pawit-vetcare/internal/domain"
)

func TestHealthIsPublic(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("expected security headers to be applied")
	}
}

func TestAPIRequiresAuthWhenDevAuthDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.AllowDevAuth = false
	server := NewServer(cfg, domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestAPIAllowsTenantScopedDevAuth(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}

func testConfig() config.Config {
	return config.Config{
		Environment:      "test",
		Port:             "8080",
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowDevAuth:     true,
		RateLimitRPM:     100,
		RequestBodyLimit: 1 << 20,
	}
}
