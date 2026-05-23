package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestPetsEndpointAliasesPetRecords(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/pets", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}

func TestProductSpecMatchesApprovedV1Scope(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/product-spec", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var payload struct {
		PaymentProvider string `json:"paymentProvider"`
		Currency        string `json:"currency"`
		GroomingEnabled bool   `json:"groomingEnabled"`
		Telemedicine    struct {
			Mode string `json:"mode"`
		} `json:"telemedicine"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.PaymentProvider != "stripe" {
		t.Fatalf("expected stripe payment provider, got %q", payload.PaymentProvider)
	}
	if payload.Currency != "USD" {
		t.Fatalf("expected USD currency, got %q", payload.Currency)
	}
	if payload.GroomingEnabled {
		t.Fatal("grooming should be out of scope for v1")
	}
	if payload.Telemedicine.Mode != "manual_meeting_link" {
		t.Fatalf("expected manual meeting links, got %q", payload.Telemedicine.Mode)
	}
}

func TestRolePoliciesExposePetParentAndLabTechnicianRules(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/role-policies", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var payload struct {
		Items []domain.RolePolicy `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	roles := map[domain.Role]domain.RolePolicy{}
	for _, item := range payload.Items {
		roles[item.Role] = item
	}

	if _, ok := roles[domain.RolePetParent]; !ok {
		t.Fatal("expected PetParent role policy")
	}
	if _, ok := roles[domain.RoleLabTechnician]; !ok {
		t.Fatal("expected LabTechnician role policy")
	}
	if hasPermission(roles[domain.RoleLabTechnician], domain.PermissionInvoiceManage) {
		t.Fatal("LabTechnician should not manage invoices")
	}
	if !hasPermission(roles[domain.RolePetParent], domain.PermissionAppointmentRequestOwn) {
		t.Fatal("PetParent should be able to request own appointments")
	}
}

func TestCreateAppointmentAllowsPetParentRequests(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{
		"locationId": "loc_001",
		"petId": "pet_001",
		"type": "telemedicine",
		"reason": "Skin follow-up"
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.AppointmentMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Appointment.Status != domain.AppointmentRequested {
		t.Fatalf("expected requested appointment, got %q", payload.Appointment.Status)
	}
}

func TestCreateAppointmentRejectsUnsupportedRole(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001","type":"in_clinic","reason":"Visit"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/appointments", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleLabTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestCancelAppointmentRequiresReason(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/apt_001/cancel", strings.NewReader(`{}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func hasPermission(policy domain.RolePolicy, permission domain.Permission) bool {
	for _, item := range policy.Permissions {
		if item == permission {
			return true
		}
	}
	return false
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
