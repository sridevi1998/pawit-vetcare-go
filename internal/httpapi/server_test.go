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

func TestCreatePetAllowsPetParent(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{
		"locationId": "loc_001",
		"name": "Nala",
		"species": "cat",
		"breed": "Domestic Shorthair",
		"guardianName": "Avery Parker",
		"guardianEmail": "avery@example.com"
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pets", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.PetMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Pet.PetName != "Nala" {
		t.Fatalf("expected created pet name, got %q", payload.Pet.PetName)
	}
}

func TestCreatePetRejectsUnsupportedSpecies(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","name":"Kiwi","species":"bird","guardianName":"Avery Parker"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pets", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestArchivePetRequiresStaffRecordPermission(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pets/pet_001/archive", strings.NewReader(`{"reason":"duplicate record"}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestUploadPetDocumentAllowsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{
		"title": "Rabies certificate",
		"documentType": "vaccine_history",
		"objectPath": "tenant_test/pets/pet_001/rabies.pdf",
		"contentType": "application/pdf",
		"sizeBytes": 1024
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pets/pet_001/documents", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.PetDocumentMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Document.DocumentType != "vaccine_history" {
		t.Fatalf("expected vaccine_history document, got %q", payload.Document.DocumentType)
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

func TestRegisterWalkInAllowsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001","reason":"Walk-in limping","priority":"urgent"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/queue/walk-ins", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.QueueMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.QueueEntry.Status != domain.QueueWaiting {
		t.Fatalf("expected waiting queue entry, got %q", payload.QueueEntry.Status)
	}
}

func TestCallQueueEntryRejectsPetParent(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/queue/queue_001/call", strings.NewReader(`{}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestCompleteQueueEntryAllowsQueueManager(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/queue/queue_001/complete", strings.NewReader(`{}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVeterinarian))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestCreateLabOrderAllowsVeterinarian(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001","testType":"CBC","sampleType":"blood","priority":"normal"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/lab-tests", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVeterinarian))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.LabOrderMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.LabTest.Status != domain.LabOrdered {
		t.Fatalf("expected ordered lab test, got %q", payload.LabTest.Status)
	}
}

func TestCreateLabOrderRejectsPetParent(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001","testType":"CBC"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/lab-tests", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestUpdateLabOrderStatusAllowsLabTechnician(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/lab-tests/lab_001/status", strings.NewReader(`{"status":"in_progress"}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleLabTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestUploadLabResultPreventsLabTechnicianSharing(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"resultNotes":"Normal CBC","reportObjectPath":"tenant/labs/cbc.pdf","shareWithPetParent":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/lab-tests/lab_001/report", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleLabTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestUploadLabResultAllowsVeterinarianSharing(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"resultNotes":"Normal CBC","reportObjectPath":"tenant/labs/cbc.pdf","shareWithPetParent":true,"markOrderCompleted":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/lab-tests/lab_001/report", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVeterinarian))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.LabOrderMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.LabTest.SharedWithPetParent {
		t.Fatal("expected lab result to be shared with pet parent")
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
