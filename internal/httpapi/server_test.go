package httpapi

import (
	"context"
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
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected generated request ID response header")
	}
}

func TestRequestIDHeaderIsEchoed(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("X-Request-ID", "request_test_123")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") != "request_test_123" {
		t.Fatalf("expected request ID to be echoed, got %q", response.Header().Get("X-Request-ID"))
	}
}

func TestCORSExposesOperationalResponseHeaders(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "http://localhost:3000")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Header().Get("Access-Control-Expose-Headers") != "Retry-After, X-Request-ID" {
		t.Fatalf("expected exposed operational headers, got %q", response.Header().Get("Access-Control-Expose-Headers"))
	}
}

func TestRecovererReturnsRequestIDOnPanic(t *testing.T) {
	cfg := testConfig()
	server := &Server{cfg: cfg}
	handler := server.recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected request ID header on recovered panic response")
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
	request.Header.Set("X-PawIt-Role", string(domain.RoleClinicAdmin))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
}

func TestLoginIssuesRoleScopedSessionCookie(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"hospitalId":"HOSP-001","email":"doctor@pawit.example","password":"pawit-demo","role":"Veterinarian"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	cookie := response.Result().Cookies()[0]
	if cookie.Name != "pawit_access" || cookie.Value == "" || !cookie.HttpOnly {
		t.Fatalf("expected http-only pawit_access cookie, got %#v", cookie)
	}

	var session domain.AuthSession
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if session.Role != domain.RoleVeterinarian || session.UserID != "user_demo_doctor" || session.Token == "" {
		t.Fatalf("unexpected session %#v", session)
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	meRequest.AddCookie(cookie)
	meResponse := httptest.NewRecorder()
	server.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("expected cookie-authenticated /me status %d, got %d: %s", http.StatusOK, meResponse.Code, meResponse.Body.String())
	}
}

func TestLoginRejectsUnassignedRole(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"hospitalId":"HOSP-001","email":"doctor@pawit.example","password":"pawit-demo","role":"ClinicAdmin"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, response.Code, response.Body.String())
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	cookie := response.Result().Cookies()[0]
	if cookie.Name != "pawit_access" || cookie.MaxAge >= 0 || cookie.Value != "" {
		t.Fatalf("expected expired pawit_access cookie, got %#v", cookie)
	}
}

func TestClientIPIgnoresForwardedHeadersFromUntrustedRemote(t *testing.T) {
	cfg := testConfig()
	server := &Server{cfg: cfg}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	request.RemoteAddr = "203.0.113.20:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.30")

	if got := server.clientIP(request); got != "203.0.113.20" {
		t.Fatalf("expected remote address, got %q", got)
	}
}

func TestClientIPUsesForwardedHeaderFromTrustedProxy(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxyCIDRs = []string{"10.0.0.0/8"}
	server := &Server{cfg: cfg}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	request.RemoteAddr = "10.2.3.4:443"
	request.Header.Set("X-Forwarded-For", "198.51.100.30, 10.2.3.4")

	if got := server.clientIP(request); got != "198.51.100.30" {
		t.Fatalf("expected forwarded client address, got %q", got)
	}
}

func TestClientIPFallsBackWhenTrustedForwardedHeaderIsInvalid(t *testing.T) {
	cfg := testConfig()
	cfg.TrustedProxyCIDRs = []string{"10.2.3.4"}
	server := &Server{cfg: cfg}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
	request.RemoteAddr = "10.2.3.4:443"
	request.Header.Set("X-Forwarded-For", "not-an-ip")

	if got := server.clientIP(request); got != "10.2.3.4" {
		t.Fatalf("expected trusted proxy remote address fallback, got %q", got)
	}
}

func TestRateLimitReturnsContractErrorEnvelope(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimitRPM = 1
	server := NewServer(cfg, domain.NewDemoStore())

	for attempt := 1; attempt <= 2; attempt++ {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/patients", nil)
		request.RemoteAddr = "203.0.113.20:443"
		request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
		request.Header.Set("X-PawIt-Role", string(domain.RoleClinicAdmin))
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		if attempt == 1 && response.Code != http.StatusOK {
			t.Fatalf("first request expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
		}
		if attempt == 2 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("second request expected status %d, got %d: %s", http.StatusTooManyRequests, response.Code, response.Body.String())
			}
			if response.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After header on rate limit response")
			}

			var payload map[string]map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode rate limit response: %v", err)
			}
			if payload["error"]["code"] != "rate_limited" {
				t.Fatalf("expected rate_limited error code, got %#v", payload)
			}
		}
	}
}

func TestPetsEndpointAliasesPetRecords(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/pets", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleClinicAdmin))
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

func TestPetDocumentsAllowsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/pets/pet_001/documents", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	var payload struct {
		Items []domain.PetDocument `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected at least one pet document")
	}
	if payload.Items[0].PetID != "pet_001" {
		t.Fatalf("expected pet_001 document, got %q", payload.Items[0].PetID)
	}
}

func TestArchivePetDocumentAllowsRecordManager(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pets/pet_001/documents/doc_001/archive", strings.NewReader(`{"reason":"duplicate upload"}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	var payload domain.PetDocumentMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Document.Status != "archived" {
		t.Fatalf("expected archived document, got %q", payload.Document.Status)
	}
}

func TestArchivePetDocumentRejectsPetParent(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/pets/pet_001/documents/doc_001/archive", strings.NewReader(`{"reason":"duplicate upload"}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestAuditLogsAllowClinicAdmin(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleClinicAdmin))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	var payload struct {
		Items []domain.AuditLogEntry `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatal("expected at least one audit log")
	}
	if payload.Items[0].Action == "" {
		t.Fatal("expected audit log action")
	}
}

func TestAuditLogsRejectReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestCreatePrescriptionAllowsVetTechnicianDraft(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{
		"locationId": "loc_001",
		"petId": "pet_001",
		"instructions": "Give with food and call clinic if vomiting occurs.",
		"medications": [
			{"medicationName": "Cetirizine", "dosage": "weight based", "frequency": "daily", "duration": "5 days"}
		]
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVetTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.PrescriptionMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Prescription.Status != "draft" {
		t.Fatalf("expected draft prescription, got %q", payload.Prescription.Status)
	}
	if len(payload.Prescription.MedicationNames) != 1 || payload.Prescription.MedicationNames[0] != "Cetirizine" {
		t.Fatalf("unexpected medications %#v", payload.Prescription.MedicationNames)
	}
}

func TestCreatePrescriptionRejectsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001","medications":[{"medicationName":"Cetirizine"}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestCreateClinicalNoteAllowsVetTechnicianDraft(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{
		"locationId": "loc_001",
		"petId": "pet_001",
		"reasonForVisit": "Annual wellness exam",
		"subjective": "Eating normally",
		"objective": "Bright and alert",
		"vitals": {"weightKg": 18.4}
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-notes", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVetTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.ClinicalNoteMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ClinicalNote.Status != "draft" {
		t.Fatalf("expected draft clinical note, got %q", payload.ClinicalNote.Status)
	}
	if payload.ClinicalNote.Subject != "Annual wellness exam" {
		t.Fatalf("expected reason subject, got %q", payload.ClinicalNote.Subject)
	}
}

func TestCreateClinicalNoteRejectsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001","reasonForVisit":"Annual wellness exam"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-notes", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestCreateClinicalNoteRequiresClinicalContent(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","petId":"pet_001"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-notes", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVeterinarian))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestFinalizeClinicalNoteAllowsVeterinarian(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-notes/note_001/finalize", strings.NewReader(`{"shareWithPetParent":true}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVeterinarian))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	var payload domain.ClinicalNoteMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ClinicalNote.Status != "finalized" {
		t.Fatalf("expected finalized clinical note, got %q", payload.ClinicalNote.Status)
	}
	if !payload.ClinicalNote.SharedWithPetParent {
		t.Fatal("expected finalized clinical note to be shared")
	}
}

func TestFinalizeClinicalNoteRejectsVetTechnician(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/clinical-notes/note_001/finalize", strings.NewReader(`{}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVetTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestFinalizePrescriptionAllowsVeterinarian(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/rx_001/finalize", strings.NewReader(`{"shareWithPetParent":true}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVeterinarian))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	var payload domain.PrescriptionMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Prescription.Status != "finalized" {
		t.Fatalf("expected finalized prescription, got %q", payload.Prescription.Status)
	}
	if !payload.Prescription.SharedWithPetParent {
		t.Fatal("expected finalized prescription to be shared")
	}
}

func TestFinalizePrescriptionRejectsVetTechnician(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/prescriptions/rx_001/finalize", strings.NewReader(`{}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleVetTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
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

func TestLocationsAllowPetParentAppointmentRequests(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/locations", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
}

func TestLocationsRejectRoleWithoutLocationWorkflow(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/locations", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", "BillingStaff")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
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

func TestCancelAppointmentReturnsContractAppointmentShape(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/appointments/apt_001/cancel", strings.NewReader(`{"reason":"guardian requested cancellation"}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}

	var payload domain.AppointmentMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Appointment.Type == "" {
		t.Fatal("expected appointment type to satisfy contract enum")
	}
	if payload.Appointment.AdditionalVeterinarians == nil {
		t.Fatal("expected additionalVeterinarians array, got nil")
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

func TestPetParentSharedMedicalReadsForwardActorScope(t *testing.T) {
	store := &readScopeRecordingStore{}
	server := NewServer(testConfig(), store)

	tests := []struct {
		name string
		path string
		call string
	}{
		{name: "prescriptions", path: "/api/v1/prescriptions", call: "Prescriptions"},
		{name: "clinical notes", path: "/api/v1/clinical-notes", call: "ClinicalNotes"},
		{name: "lab tests", path: "/api/v1/lab-tests", call: "LabTests"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.calls = nil
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
			request.Header.Set("X-PawIt-User-ID", "guardian_user_001")
			request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
			}
			if len(store.calls) != 1 {
				t.Fatalf("expected one read call, got %#v", store.calls)
			}
			call := store.calls[0]
			if call.name != tt.call || call.actorUserID != "guardian_user_001" || call.actorRole != domain.RolePetParent {
				t.Fatalf("unexpected read scope %#v", call)
			}
		})
	}
}

func TestPetParentAppointmentReadsForwardActorScope(t *testing.T) {
	store := &readScopeRecordingStore{}
	server := NewServer(testConfig(), store)

	tests := []struct {
		name string
		path string
		call string
	}{
		{name: "appointments", path: "/api/v1/appointments", call: "Appointments"},
		{name: "calendar", path: "/api/v1/calendar", call: "Calendar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.calls = nil
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
			request.Header.Set("X-PawIt-User-ID", "guardian_user_001")
			request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
			}
			if len(store.calls) != 1 {
				t.Fatalf("expected one read call, got %#v", store.calls)
			}
			call := store.calls[0]
			if call.name != tt.call || call.actorUserID != "guardian_user_001" || call.actorRole != domain.RolePetParent {
				t.Fatalf("unexpected read scope %#v", call)
			}
		})
	}
}

func TestPetParentPetRecordReadsForwardActorScope(t *testing.T) {
	store := &readScopeRecordingStore{}
	server := NewServer(testConfig(), store)

	tests := []struct {
		name string
		path string
		call string
	}{
		{name: "pets", path: "/api/v1/pets", call: "Patients"},
		{name: "pet documents", path: "/api/v1/pets/pet_001/documents", call: "PetDocuments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store.calls = nil
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
			request.Header.Set("X-PawIt-User-ID", "guardian_user_001")
			request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
			}
			if len(store.calls) != 1 {
				t.Fatalf("expected one read call, got %#v", store.calls)
			}
			call := store.calls[0]
			if call.name != tt.call || call.actorUserID != "guardian_user_001" || call.actorRole != domain.RolePetParent {
				t.Fatalf("unexpected read scope %#v", call)
			}
		})
	}
}

func TestPetParentBillingReadsForwardActorScope(t *testing.T) {
	store := &readScopeRecordingStore{}
	server := NewServer(testConfig(), store)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/billing", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-User-ID", "guardian_user_001")
	request.Header.Set("X-PawIt-Role", string(domain.RolePetParent))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if len(store.calls) != 1 {
		t.Fatalf("expected one read call, got %#v", store.calls)
	}
	call := store.calls[0]
	if call.name != "Billing" || call.actorUserID != "guardian_user_001" || call.actorRole != domain.RolePetParent {
		t.Fatalf("unexpected read scope %#v", call)
	}
}

func TestPatientsRejectsRoleWithoutReadPermission(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/pets", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", "BillingStaff")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestBillingRejectsRoleWithoutReadPermission(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/billing", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleLabTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestClinicalNotesRejectsRoleWithoutReadPermission(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/clinical-notes", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", "BillingStaff")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestLabTestsRejectsRoleWithoutReadPermission(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/lab-tests", nil)
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", "BillingStaff")
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
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

func TestCreateInvoiceAllowsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{
		"locationId": "loc_001",
		"petId": "pet_001",
		"lineItems": [
			{"description": "Wellness exam", "quantity": 1, "unitAmountCents": 6500},
			{"description": "Rabies vaccine", "quantity": 1, "unitAmountCents": 3200}
		],
		"taxCents": 485
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/invoices", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.InvoiceMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Invoice.Amount != 10185 {
		t.Fatalf("expected invoice amount 10185, got %d", payload.Invoice.Amount)
	}
	if payload.Invoice.Status != "issued" {
		t.Fatalf("expected issued invoice, got %q", payload.Invoice.Status)
	}
}

func TestCreateInvoiceRejectsLabTechnician(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"locationId":"loc_001","lineItems":[{"description":"CBC","quantity":1,"unitAmountCents":4500}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/invoices", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleLabTechnician))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestVoidInvoiceRequiresClinicAdminPermission(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/invoices/inv_001/void", strings.NewReader(`{"reason":"billing error"}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestVoidInvoiceRequiresReason(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/billing/invoices/inv_001/void", strings.NewReader(`{}`))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleClinicAdmin))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestCreateStaffAllowsClinicAdmin(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"name":"Priya Shah","email":"priya@pawit.example","role":"Receptionist"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/staff", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleClinicAdmin))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, response.Code, response.Body.String())
	}

	var payload domain.StaffMutationResult
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.StaffMember.Email != "priya@pawit.example" {
		t.Fatalf("expected staff email, got %q", payload.StaffMember.Email)
	}
	if payload.StaffMember.Role != string(domain.RoleReceptionist) {
		t.Fatalf("expected receptionist role, got %q", payload.StaffMember.Role)
	}
	if payload.StaffMember.Status != "invited" {
		t.Fatalf("expected invited status, got %q", payload.StaffMember.Status)
	}
}

func TestCreateStaffRejectsReceptionist(t *testing.T) {
	server := NewServer(testConfig(), domain.NewDemoStore())
	body := `{"name":"Priya Shah","email":"priya@pawit.example","role":"Receptionist"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/staff", strings.NewReader(body))
	request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
	request.Header.Set("X-PawIt-Role", string(domain.RoleReceptionist))
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestMutationEndpointsForwardIdempotencyKey(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		role domain.Role
		call string
	}{
		{
			name: "create appointment",
			path: "/api/v1/appointments",
			body: `{"locationId":"loc_001","petId":"pet_001","type":"in_clinic","reason":"Wellness visit"}`,
			role: domain.RoleReceptionist,
			call: "CreateAppointment",
		},
		{
			name: "cancel appointment",
			path: "/api/v1/appointments/apt_001/cancel",
			body: `{"reason":"guardian requested cancellation"}`,
			role: domain.RoleReceptionist,
			call: "CancelAppointment",
		},
		{
			name: "register walk in",
			path: "/api/v1/queue/walk-ins",
			body: `{"locationId":"loc_001","petId":"pet_001","reason":"Walk-in limping","priority":"urgent"}`,
			role: domain.RoleReceptionist,
			call: "RegisterWalkIn",
		},
		{
			name: "call queue entry",
			path: "/api/v1/queue/queue_001/call",
			body: `{}`,
			role: domain.RoleReceptionist,
			call: "UpdateQueueStatus:called",
		},
		{
			name: "start queue entry",
			path: "/api/v1/queue/queue_001/start",
			body: `{}`,
			role: domain.RoleReceptionist,
			call: "UpdateQueueStatus:in_progress",
		},
		{
			name: "complete queue entry",
			path: "/api/v1/queue/queue_001/complete",
			body: `{}`,
			role: domain.RoleVeterinarian,
			call: "UpdateQueueStatus:completed",
		},
		{
			name: "cancel queue entry",
			path: "/api/v1/queue/queue_001/cancel",
			body: `{}`,
			role: domain.RoleReceptionist,
			call: "UpdateQueueStatus:cancelled",
		},
		{
			name: "create pet",
			path: "/api/v1/pets",
			body: `{"locationId":"loc_001","name":"Nala","species":"cat","guardianName":"Avery Parker","guardianEmail":"avery@example.com"}`,
			role: domain.RolePetParent,
			call: "CreatePet",
		},
		{
			name: "archive pet",
			path: "/api/v1/pets/pet_001/archive",
			body: `{"reason":"duplicate record"}`,
			role: domain.RoleReceptionist,
			call: "ArchivePet",
		},
		{
			name: "upload pet document",
			path: "/api/v1/pets/pet_001/documents",
			body: `{"title":"Rabies certificate","documentType":"vaccine_history","objectPath":"tenant_test/pets/pet_001/rabies.pdf","contentType":"application/pdf","sizeBytes":1024}`,
			role: domain.RoleReceptionist,
			call: "UploadPetDocument",
		},
		{
			name: "archive pet document",
			path: "/api/v1/pets/pet_001/documents/doc_001/archive",
			body: `{"reason":"duplicate upload"}`,
			role: domain.RoleReceptionist,
			call: "ArchivePetDocument",
		},
		{
			name: "create prescription",
			path: "/api/v1/prescriptions",
			body: `{"locationId":"loc_001","petId":"pet_001","instructions":"Give with food.","medications":[{"medicationName":"Cetirizine"}]}`,
			role: domain.RoleVetTechnician,
			call: "CreatePrescription",
		},
		{
			name: "finalize prescription",
			path: "/api/v1/prescriptions/rx_001/finalize",
			body: `{"shareWithPetParent":true}`,
			role: domain.RoleVeterinarian,
			call: "FinalizePrescription",
		},
		{
			name: "create clinical note",
			path: "/api/v1/clinical-notes",
			body: `{"locationId":"loc_001","petId":"pet_001","reasonForVisit":"Annual wellness exam"}`,
			role: domain.RoleVetTechnician,
			call: "CreateClinicalNote",
		},
		{
			name: "finalize clinical note",
			path: "/api/v1/clinical-notes/note_001/finalize",
			body: `{"shareWithPetParent":true}`,
			role: domain.RoleVeterinarian,
			call: "FinalizeClinicalNote",
		},
		{
			name: "create lab order",
			path: "/api/v1/lab-tests",
			body: `{"locationId":"loc_001","petId":"pet_001","testType":"CBC","sampleType":"blood","priority":"normal"}`,
			role: domain.RoleVeterinarian,
			call: "CreateLabOrder",
		},
		{
			name: "update lab order status",
			path: "/api/v1/lab-tests/lab_001/status",
			body: `{"status":"in_progress"}`,
			role: domain.RoleLabTechnician,
			call: "UpdateLabOrderStatus",
		},
		{
			name: "upload lab result",
			path: "/api/v1/lab-tests/lab_001/report",
			body: `{"resultNotes":"Normal CBC","reportObjectPath":"tenant/labs/cbc.pdf","shareWithPetParent":true,"markOrderCompleted":true}`,
			role: domain.RoleVeterinarian,
			call: "UploadLabResult",
		},
		{
			name: "create invoice",
			path: "/api/v1/billing/invoices",
			body: `{"locationId":"loc_001","petId":"pet_001","lineItems":[{"description":"Wellness exam","quantity":1,"unitAmountCents":6500}]}`,
			role: domain.RoleReceptionist,
			call: "CreateInvoice",
		},
		{
			name: "void invoice",
			path: "/api/v1/billing/invoices/inv_001/void",
			body: `{"reason":"billing error"}`,
			role: domain.RoleClinicAdmin,
			call: "VoidInvoice",
		},
		{
			name: "create staff",
			path: "/api/v1/staff",
			body: `{"name":"Priya Shah","email":"priya@pawit.example","role":"Receptionist"}`,
			role: domain.RoleClinicAdmin,
			call: "CreateStaff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newIdempotencyRecordingStore()
			server := NewServer(testConfig(), store)
			request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			request.Header.Set("X-PawIt-Tenant-ID", "tenant_test")
			request.Header.Set("X-PawIt-Role", string(tt.role))
			request.Header.Set("Idempotency-Key", " idem-key-001 ")
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code < http.StatusOK || response.Code >= http.StatusMultipleChoices {
				t.Fatalf("expected success status, got %d: %s", response.Code, response.Body.String())
			}
			if got := store.keys[tt.call]; got != "idem-key-001" {
				t.Fatalf("expected %s to receive idempotency key %q, got %q", tt.call, "idem-key-001", got)
			}
		})
	}
}

type idempotencyRecordingStore struct {
	domain.DemoStore
	keys map[string]string
}

func newIdempotencyRecordingStore() *idempotencyRecordingStore {
	return &idempotencyRecordingStore{
		DemoStore: domain.NewDemoStore(),
		keys:      map[string]string{},
	}
}

func (s *idempotencyRecordingStore) record(name string, key string) {
	s.keys[name] = key
}

func (s *idempotencyRecordingStore) CreateAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateAppointmentInput, idempotencyKey string) (domain.AppointmentMutationResult, error) {
	s.record("CreateAppointment", idempotencyKey)
	return s.DemoStore.CreateAppointment(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CancelAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, appointmentID string, input domain.CancelAppointmentInput, idempotencyKey string) (domain.AppointmentMutationResult, error) {
	s.record("CancelAppointment", idempotencyKey)
	return s.DemoStore.CancelAppointment(ctx, tenantID, actorUserID, actorRole, appointmentID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) RegisterWalkIn(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.RegisterWalkInInput, idempotencyKey string) (domain.QueueMutationResult, error) {
	s.record("RegisterWalkIn", idempotencyKey)
	return s.DemoStore.RegisterWalkIn(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) UpdateQueueStatus(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, queueID string, status domain.QueueStatus, input domain.UpdateQueueInput, idempotencyKey string) (domain.QueueMutationResult, error) {
	s.record("UpdateQueueStatus:"+string(status), idempotencyKey)
	return s.DemoStore.UpdateQueueStatus(ctx, tenantID, actorUserID, actorRole, queueID, status, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CreatePet(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreatePetInput, idempotencyKey string) (domain.PetMutationResult, error) {
	s.record("CreatePet", idempotencyKey)
	return s.DemoStore.CreatePet(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) ArchivePet(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, input domain.ArchivePetInput, idempotencyKey string) (domain.PetMutationResult, error) {
	s.record("ArchivePet", idempotencyKey)
	return s.DemoStore.ArchivePet(ctx, tenantID, actorUserID, actorRole, petID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) UploadPetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, input domain.UploadPetDocumentInput, idempotencyKey string) (domain.PetDocumentMutationResult, error) {
	s.record("UploadPetDocument", idempotencyKey)
	return s.DemoStore.UploadPetDocument(ctx, tenantID, actorUserID, actorRole, petID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) ArchivePetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, documentID string, input domain.ArchivePetDocumentInput, idempotencyKey string) (domain.PetDocumentMutationResult, error) {
	s.record("ArchivePetDocument", idempotencyKey)
	return s.DemoStore.ArchivePetDocument(ctx, tenantID, actorUserID, actorRole, petID, documentID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CreatePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreatePrescriptionInput, idempotencyKey string) (domain.PrescriptionMutationResult, error) {
	s.record("CreatePrescription", idempotencyKey)
	return s.DemoStore.CreatePrescription(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) FinalizePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, prescriptionID string, input domain.FinalizePrescriptionInput, idempotencyKey string) (domain.PrescriptionMutationResult, error) {
	s.record("FinalizePrescription", idempotencyKey)
	return s.DemoStore.FinalizePrescription(ctx, tenantID, actorUserID, actorRole, prescriptionID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CreateClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateClinicalNoteInput, idempotencyKey string) (domain.ClinicalNoteMutationResult, error) {
	s.record("CreateClinicalNote", idempotencyKey)
	return s.DemoStore.CreateClinicalNote(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) FinalizeClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, clinicalNoteID string, input domain.FinalizeClinicalNoteInput, idempotencyKey string) (domain.ClinicalNoteMutationResult, error) {
	s.record("FinalizeClinicalNote", idempotencyKey)
	return s.DemoStore.FinalizeClinicalNote(ctx, tenantID, actorUserID, actorRole, clinicalNoteID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CreateLabOrder(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateLabOrderInput, idempotencyKey string) (domain.LabOrderMutationResult, error) {
	s.record("CreateLabOrder", idempotencyKey)
	return s.DemoStore.CreateLabOrder(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) UpdateLabOrderStatus(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, labOrderID string, input domain.UpdateLabOrderStatusInput, idempotencyKey string) (domain.LabOrderMutationResult, error) {
	s.record("UpdateLabOrderStatus", idempotencyKey)
	return s.DemoStore.UpdateLabOrderStatus(ctx, tenantID, actorUserID, actorRole, labOrderID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) UploadLabResult(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, labOrderID string, input domain.UploadLabResultInput, idempotencyKey string) (domain.LabOrderMutationResult, error) {
	s.record("UploadLabResult", idempotencyKey)
	return s.DemoStore.UploadLabResult(ctx, tenantID, actorUserID, actorRole, labOrderID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CreateInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateInvoiceInput, idempotencyKey string) (domain.InvoiceMutationResult, error) {
	s.record("CreateInvoice", idempotencyKey)
	return s.DemoStore.CreateInvoice(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) VoidInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, invoiceID string, input domain.VoidInvoiceInput, idempotencyKey string) (domain.InvoiceMutationResult, error) {
	s.record("VoidInvoice", idempotencyKey)
	return s.DemoStore.VoidInvoice(ctx, tenantID, actorUserID, actorRole, invoiceID, input, idempotencyKey)
}

func (s *idempotencyRecordingStore) CreateStaff(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateStaffInput, idempotencyKey string) (domain.StaffMutationResult, error) {
	s.record("CreateStaff", idempotencyKey)
	return s.DemoStore.CreateStaff(ctx, tenantID, actorUserID, actorRole, input, idempotencyKey)
}

type recordedReadScopeCall struct {
	name        string
	actorUserID string
	actorRole   domain.Role
}

type readScopeRecordingStore struct {
	domain.DemoStore
	calls []recordedReadScopeCall
}

func (s *readScopeRecordingStore) Patients(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.PatientRecord, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "Patients", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.Patients(ctx, tenantID, actorUserID, actorRole)
}

func (s *readScopeRecordingStore) Appointments(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.Appointment, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "Appointments", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.Appointments(ctx, tenantID, actorUserID, actorRole)
}

func (s *readScopeRecordingStore) Calendar(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) (map[string]any, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "Calendar", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.Calendar(ctx, tenantID, actorUserID, actorRole)
}

func (s *readScopeRecordingStore) PetDocuments(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string) ([]domain.PetDocument, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "PetDocuments", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.PetDocuments(ctx, tenantID, actorUserID, actorRole, petID)
}

func (s *readScopeRecordingStore) Billing(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) (map[string]any, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "Billing", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.Billing(ctx, tenantID, actorUserID, actorRole)
}

func (s *readScopeRecordingStore) Prescriptions(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.Prescription, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "Prescriptions", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.Prescriptions(ctx, tenantID, actorUserID, actorRole)
}

func (s *readScopeRecordingStore) ClinicalNotes(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.ClinicalNote, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "ClinicalNotes", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.ClinicalNotes(ctx, tenantID, actorUserID, actorRole)
}

func (s *readScopeRecordingStore) LabTests(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.LabTest, error) {
	s.calls = append(s.calls, recordedReadScopeCall{name: "LabTests", actorUserID: actorUserID, actorRole: actorRole})
	return s.DemoStore.LabTests(ctx, tenantID, actorUserID, actorRole)
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
		Environment:       "test",
		Port:              "8080",
		AllowedOrigins:    []string{"http://localhost:3000"},
		TrustedProxyCIDRs: nil,
		AllowDevAuth:      true,
		RateLimitRPM:      100,
		RequestBodyLimit:  1 << 20,
	}
}
