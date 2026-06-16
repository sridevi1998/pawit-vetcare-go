package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pawit-vetcare/internal/config"
	"pawit-vetcare/internal/domain"
)

type Server struct {
	cfg     config.Config
	store   domain.Store
	mux     *http.ServeMux
	limiter *rateLimiter
}

func NewServer(cfg config.Config, store domain.Store) http.Handler {
	s := &Server{
		cfg:     cfg,
		store:   store,
		mux:     http.NewServeMux(),
		limiter: newRateLimiter(cfg.RateLimitRPM, cfg.RateWindow()),
	}
	s.routes()
	return s.middleware(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.login)
	s.mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	s.mux.HandleFunc("GET /api/v1/me", s.me)
	s.mux.HandleFunc("GET /api/v1/product-spec", s.productSpec)
	s.mux.HandleFunc("GET /api/v1/role-policies", s.rolePolicies)
	s.mux.HandleFunc("GET /api/v1/navigation", s.navigation)
	s.mux.HandleFunc("GET /api/v1/locations", s.locations)
	s.mux.HandleFunc("GET /api/v1/dashboard/summary", s.summary)
	s.mux.HandleFunc("GET /api/v1/appointments", s.appointments)
	s.mux.HandleFunc("POST /api/v1/appointments", s.createAppointment)
	s.mux.HandleFunc("POST /api/v1/appointments/{id}/cancel", s.cancelAppointment)
	s.mux.HandleFunc("GET /api/v1/calendar", s.calendar)
	s.mux.HandleFunc("GET /api/v1/queue", s.queue)
	s.mux.HandleFunc("POST /api/v1/queue/walk-ins", s.registerWalkIn)
	s.mux.HandleFunc("POST /api/v1/queue/{id}/call", s.callQueueEntry)
	s.mux.HandleFunc("POST /api/v1/queue/{id}/start", s.startQueueEntry)
	s.mux.HandleFunc("POST /api/v1/queue/{id}/complete", s.completeQueueEntry)
	s.mux.HandleFunc("POST /api/v1/queue/{id}/cancel", s.cancelQueueEntry)
	s.mux.HandleFunc("GET /api/v1/pets", s.patients)
	s.mux.HandleFunc("POST /api/v1/pets", s.createPet)
	s.mux.HandleFunc("POST /api/v1/pets/{id}/archive", s.archivePet)
	s.mux.HandleFunc("GET /api/v1/pets/{id}/documents", s.petDocuments)
	s.mux.HandleFunc("POST /api/v1/pets/{id}/documents/upload-url", s.preparePetDocumentUpload)
	s.mux.HandleFunc("POST /api/v1/pets/{id}/documents", s.uploadPetDocument)
	s.mux.HandleFunc("POST /api/v1/pets/{id}/documents/{documentId}/download-url", s.createPetDocumentDownload)
	s.mux.HandleFunc("POST /api/v1/pets/{id}/documents/{documentId}/archive", s.archivePetDocument)
	s.mux.HandleFunc("GET /api/v1/patients", s.patients)
	s.mux.HandleFunc("GET /api/v1/prescriptions", s.prescriptions)
	s.mux.HandleFunc("POST /api/v1/prescriptions", s.createPrescription)
	s.mux.HandleFunc("POST /api/v1/prescriptions/{id}/finalize", s.finalizePrescription)
	s.mux.HandleFunc("GET /api/v1/prescription-templates", s.prescriptionTemplates)
	s.mux.HandleFunc("GET /api/v1/clinical-notes", s.clinicalNotes)
	s.mux.HandleFunc("POST /api/v1/clinical-notes", s.createClinicalNote)
	s.mux.HandleFunc("POST /api/v1/clinical-notes/{id}/finalize", s.finalizeClinicalNote)
	s.mux.HandleFunc("GET /api/v1/lab-tests", s.labTests)
	s.mux.HandleFunc("POST /api/v1/lab-tests", s.createLabOrder)
	s.mux.HandleFunc("POST /api/v1/lab-tests/{id}/status", s.updateLabOrderStatus)
	s.mux.HandleFunc("POST /api/v1/lab-tests/{id}/report", s.uploadLabResult)
	s.mux.HandleFunc("GET /api/v1/billing", s.billing)
	s.mux.HandleFunc("POST /api/v1/billing/invoices", s.createInvoice)
	s.mux.HandleFunc("POST /api/v1/billing/invoices/{id}/void", s.voidInvoice)
	s.mux.HandleFunc("GET /api/v1/analytics", s.analytics)
	s.mux.HandleFunc("GET /api/v1/feedback", s.feedback)
	s.mux.HandleFunc("GET /api/v1/doctors", s.doctors)
	s.mux.HandleFunc("GET /api/v1/staff", s.staff)
	s.mux.HandleFunc("POST /api/v1/staff", s.createStaff)
	s.mux.HandleFunc("GET /api/v1/tenants", s.tenants)
	s.mux.HandleFunc("POST /api/v1/tenants", s.createTenant)
	s.mux.HandleFunc("GET /api/v1/tenants/{id}", s.tenant)
	s.mux.HandleFunc("PATCH /api/v1/tenants/{id}", s.updateTenant)
	s.mux.HandleFunc("POST /api/v1/tenants/{id}/locations", s.createTenantLocation)
	s.mux.HandleFunc("PATCH /api/v1/tenants/{id}/locations/{locationId}", s.updateTenantLocation)
	s.mux.HandleFunc("GET /api/v1/audit-logs", s.auditLogs)
	s.mux.HandleFunc("/", s.notFound)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input domain.LoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	input.Email = strings.TrimSpace(input.Email)
	input.Password = strings.TrimSpace(input.Password)
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.HospitalID = strings.TrimSpace(input.HospitalID)
	if input.Email == "" || input.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Email and password are required.")
		return
	}

	identity, err := s.store.Authenticate(r.Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email, password, tenant, or role is invalid.")
			return
		}
		slog.Error("login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "PawIt could not complete the login request.")
		return
	}

	expiresAt := time.Now().UTC().Add(12 * time.Hour)
	auth := AuthContext{UserID: identity.UserID, TenantID: identity.TenantID, Role: string(identity.Role)}
	token, err := s.signToken(auth, expiresAt)
	if err != nil {
		slog.Error("token signing failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "PawIt could not create a login session.")
		return
	}

	http.SetCookie(w, s.authCookie(token, expiresAt))
	writeJSON(w, http.StatusOK, domain.AuthSession{
		UserID:      identity.UserID,
		TenantID:    identity.TenantID,
		Role:        identity.Role,
		DisplayName: identity.DisplayName,
		Email:       identity.Email,
		Token:       token,
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, s.expiredAuthCookie())
	writeJSON(w, http.StatusOK, map[string]any{"status": "signed_out"})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "pawit-vetcare-api"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.store.Ready(ctx); err != nil {
		slog.Warn("readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable", "PawIt API dependencies are not ready.")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "checkedAt": time.Now().UTC().Format(time.RFC3339)})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":       auth.UserID,
			"role":     auth.Role,
			"tenantId": auth.TenantID,
		},
		"clinic": map[string]any{
			"name": "PawIt VetCare",
			"type": "Veterinary Management Portal",
		},
	})
}

func (s *Server) navigation(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	sections, err := s.store.Navigation(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"sections": sections}, err)
}

func (s *Server) locations(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role,
		domain.PermissionLocationManage,
		domain.PermissionAppointmentManage,
		domain.PermissionAppointmentRequestOwn,
		domain.PermissionQueueManage,
		domain.PermissionPetRecordManage,
		domain.PermissionPetRecordManageOwn,
		domain.PermissionClinicalNoteDraft,
		domain.PermissionPrescriptionDraft,
		domain.PermissionLabOrderCreate,
		domain.PermissionLabOrderProcess,
		domain.PermissionInvoiceCreate,
		domain.PermissionStaffManage,
	) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view clinic locations.")
		return
	}
	items, err := s.store.Locations(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) productSpec(w http.ResponseWriter, r *http.Request) {
	spec, err := s.store.ProductSpec(r.Context())
	writeData(w, spec, err)
}

func (s *Server) rolePolicies(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.RolePolicies(r.Context())
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	metrics, err := s.store.Summary(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"metrics": metrics}, err)
}

func (s *Server) appointments(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionAppointmentManage, domain.PermissionAppointmentRequestOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view appointments.")
		return
	}
	items, err := s.store.Appointments(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createAppointment(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionAppointmentManage, domain.PermissionAppointmentRequestOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot create appointments.")
		return
	}

	var input domain.CreateAppointmentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateAppointment(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateAppointment(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) cancelAppointment(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionAppointmentManage, domain.PermissionAppointmentRequestOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot cancel appointments.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Appointment ID is required.")
		return
	}

	var input domain.CancelAppointmentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Cancellation reason is required.")
		return
	}

	result, err := s.store.CancelAppointment(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionAppointmentManage, domain.PermissionAppointmentRequestOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view the calendar.")
		return
	}
	payload, err := s.store.Calendar(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, payload, err)
}

func (s *Server) queue(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.Queue(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) registerWalkIn(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionQueueManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage the queue.")
		return
	}

	var input domain.RegisterWalkInInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateRegisterWalkIn(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.RegisterWalkIn(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) callQueueEntry(w http.ResponseWriter, r *http.Request) {
	s.updateQueueStatus(w, r, domain.QueueCalled)
}

func (s *Server) startQueueEntry(w http.ResponseWriter, r *http.Request) {
	s.updateQueueStatus(w, r, domain.QueueInProgress)
}

func (s *Server) completeQueueEntry(w http.ResponseWriter, r *http.Request) {
	s.updateQueueStatus(w, r, domain.QueueCompleted)
}

func (s *Server) cancelQueueEntry(w http.ResponseWriter, r *http.Request) {
	s.updateQueueStatus(w, r, domain.QueueCancelled)
}

func (s *Server) updateQueueStatus(w http.ResponseWriter, r *http.Request, status domain.QueueStatus) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionQueueManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage the queue.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Queue entry ID is required.")
		return
	}

	var input domain.UpdateQueueInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.UpdateQueueStatus(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, status, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) patients(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetRecordManage, domain.PermissionPetRecordManageOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view pet records.")
		return
	}

	items, err := s.store.Patients(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createPet(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetRecordManage, domain.PermissionPetRecordManageOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot create pet records.")
		return
	}

	var input domain.CreatePetInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreatePet(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreatePet(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) archivePet(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetRecordManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot archive pet records.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Pet ID is required.")
		return
	}

	var input domain.ArchivePetInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Archive reason is required.")
		return
	}

	result, err := s.store.ArchivePet(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) uploadPetDocument(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetDocumentUpload) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot upload pet documents.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Pet ID is required.")
		return
	}

	var input domain.UploadPetDocumentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateUploadPetDocument(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.UploadPetDocument(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) preparePetDocumentUpload(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetDocumentUpload) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot upload pet documents.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Pet ID is required.")
		return
	}

	var input domain.PreparePetDocumentUploadInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validatePreparePetDocumentUpload(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.PreparePetDocumentUpload(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) petDocuments(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetRecordManage, domain.PermissionPetRecordManageOwn, domain.PermissionPetDocumentUpload) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view pet documents.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Pet ID is required.")
		return
	}

	items, err := s.store.PetDocuments(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createPetDocumentDownload(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetRecordManage, domain.PermissionPetRecordManageOwn, domain.PermissionPetDocumentUpload) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot download pet documents.")
		return
	}

	petID := strings.TrimSpace(r.PathValue("id"))
	if petID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Pet ID is required.")
		return
	}
	documentID := strings.TrimSpace(r.PathValue("documentId"))
	if documentID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Document ID is required.")
		return
	}

	result, err := s.store.CreatePetDocumentDownload(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), petID, documentID, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) archivePetDocument(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPetRecordManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot archive pet documents.")
		return
	}

	petID := strings.TrimSpace(r.PathValue("id"))
	if petID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Pet ID is required.")
		return
	}
	documentID := strings.TrimSpace(r.PathValue("documentId"))
	if documentID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Document ID is required.")
		return
	}

	var input domain.ArchivePetDocumentInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Archive reason is required.")
		return
	}

	result, err := s.store.ArchivePetDocument(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), petID, documentID, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) prescriptionTemplates(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.PrescriptionTemplates(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) prescriptions(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPrescriptionView, domain.PermissionPrescriptionViewShared) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view prescriptions.")
		return
	}

	items, err := s.store.Prescriptions(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createPrescription(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPrescriptionDraft) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot draft prescriptions.")
		return
	}

	var input domain.CreatePrescriptionInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreatePrescription(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreatePrescription(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) finalizePrescription(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPrescriptionFinalize) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot finalize prescriptions.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Prescription ID is required.")
		return
	}

	var input domain.FinalizePrescriptionInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.FinalizePrescription(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) clinicalNotes(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionClinicalNoteView, domain.PermissionClinicalNoteViewShared) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view clinical notes.")
		return
	}

	items, err := s.store.ClinicalNotes(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createClinicalNote(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionClinicalNoteDraft) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot draft clinical notes.")
		return
	}

	var input domain.CreateClinicalNoteInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateClinicalNote(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateClinicalNote(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) finalizeClinicalNote(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionClinicalNoteFinalize) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot finalize clinical notes.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Clinical note ID is required.")
		return
	}

	var input domain.FinalizeClinicalNoteInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.FinalizeClinicalNote(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) labTests(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionLabOrderCreate, domain.PermissionLabOrderProcess, domain.PermissionLabResultShare, domain.PermissionLabResultViewShared) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view lab tests.")
		return
	}

	items, err := s.store.LabTests(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createLabOrder(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionLabOrderCreate) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot create lab orders.")
		return
	}

	var input domain.CreateLabOrderInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateLabOrder(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateLabOrder(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) updateLabOrderStatus(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionLabOrderProcess) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot process lab orders.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Lab order ID is required.")
		return
	}

	var input domain.UpdateLabOrderStatusInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if !validLabStatus(input.Status) {
		writeError(w, http.StatusBadRequest, "invalid_request", "status is not supported")
		return
	}

	result, err := s.store.UpdateLabOrderStatus(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) uploadLabResult(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionLabOrderProcess, domain.PermissionLabResultShare) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot upload lab results.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Lab order ID is required.")
		return
	}

	var input domain.UploadLabResultInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateUploadLabResult(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if input.ShareWithPetParent && !roleCan(auth.Role, domain.PermissionLabResultShare) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot share lab results with pet parents.")
		return
	}

	result, err := s.store.UploadLabResult(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) billing(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionInvoiceCreate, domain.PermissionInvoiceManage, domain.PermissionPaymentRefundVoid, domain.PermissionInvoicePayOwn) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view billing records.")
		return
	}

	payload, err := s.store.Billing(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role))
	writeData(w, payload, err)
}

func (s *Server) createInvoice(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionInvoiceCreate) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot create invoices.")
		return
	}

	var input domain.CreateInvoiceInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateInvoice(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateInvoice(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) voidInvoice(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionPaymentRefundVoid) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot void invoices.")
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invoice ID is required.")
		return
	}

	var input domain.VoidInvoiceInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Void reason is required.")
		return
	}

	result, err := s.store.VoidInvoice(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), id, input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) analytics(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	payload, err := s.store.Analytics(r.Context(), auth.TenantID)
	writeData(w, payload, err)
}

func (s *Server) feedback(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	payload, err := s.store.Feedback(r.Context(), auth.TenantID)
	writeData(w, payload, err)
}

func (s *Server) doctors(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.Doctors(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) staff(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.Staff(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) createStaff(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionStaffManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage staff.")
		return
	}

	var input domain.CreateStaffInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateStaff(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateStaff(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) tenants(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionTenantManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage tenants.")
		return
	}

	items, err := s.store.Tenants(r.Context())
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) tenant(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionTenantManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage tenants.")
		return
	}

	item, err := s.store.Tenant(r.Context(), r.PathValue("id"))
	writeMutation(w, http.StatusOK, map[string]any{"tenant": item}, err)
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionTenantManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage tenants.")
		return
	}

	var input domain.CreateTenantInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateTenant(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateTenant(r.Context(), auth.TenantID, auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionTenantManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage tenants.")
		return
	}

	var input domain.UpdateTenantInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateUpdateTenant(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.UpdateTenant(r.Context(), r.PathValue("id"), auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) createTenantLocation(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionTenantManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage tenant locations.")
		return
	}

	var input domain.CreateClinicLocationInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateCreateClinicLocation(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.CreateTenantLocation(r.Context(), r.PathValue("id"), auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusCreated, result, err)
}

func (s *Server) updateTenantLocation(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionTenantManage) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot manage tenant locations.")
		return
	}

	var input domain.UpdateClinicLocationInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := validateUpdateClinicLocation(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	result, err := s.store.UpdateTenantLocation(r.Context(), r.PathValue("id"), r.PathValue("locationId"), auth.UserID, domain.Role(auth.Role), input, idempotencyKey(r))
	writeMutation(w, http.StatusOK, result, err)
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	if !roleCan(auth.Role, domain.PermissionAuditLogView) {
		writeError(w, http.StatusForbidden, "forbidden", "This role cannot view audit logs.")
		return
	}

	items, err := s.store.AuditLogs(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "The requested PawIt endpoint does not exist.")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeData(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		slog.Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "PawIt could not complete the request.")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func writeMutation(w http.ResponseWriter, status int, payload any, err error) {
	if err == nil {
		writeJSON(w, status, payload)
		return
	}
	switch {
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		slog.Error("mutation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "PawIt could not complete the request.")
	}
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func validateCreateAppointment(input domain.CreateAppointmentInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if strings.TrimSpace(input.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return errors.New("reason is required")
	}
	if !validAppointmentType(input.Type) {
		return errors.New("type is not supported")
	}
	if input.StartsAt != nil {
		if _, err := time.Parse(time.RFC3339, *input.StartsAt); err != nil {
			return errors.New("startsAt must be RFC3339")
		}
	}
	if input.EndsAt != nil {
		if _, err := time.Parse(time.RFC3339, *input.EndsAt); err != nil {
			return errors.New("endsAt must be RFC3339")
		}
	}
	return nil
}

func validateRegisterWalkIn(input domain.RegisterWalkInInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if strings.TrimSpace(input.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return errors.New("reason is required")
	}
	return nil
}

func validateCreatePet(input domain.CreatePetInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if !validSpecies(input.Species) {
		return errors.New("species must be dog or cat")
	}
	if strings.TrimSpace(input.GuardianName) == "" {
		return errors.New("guardianName is required")
	}
	return nil
}

func validateUploadPetDocument(input domain.UploadPetDocumentInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(input.DocumentType) == "" {
		return errors.New("documentType is required")
	}
	if strings.TrimSpace(input.ObjectPath) == "" {
		return errors.New("objectPath is required")
	}
	if strings.TrimSpace(input.ContentType) == "" {
		return errors.New("contentType is required")
	}
	if input.SizeBytes < 0 {
		return errors.New("sizeBytes must be greater than or equal to 0")
	}
	return nil
}

func validatePreparePetDocumentUpload(input domain.PreparePetDocumentUploadInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(input.DocumentType) == "" {
		return errors.New("documentType is required")
	}
	if strings.TrimSpace(input.ContentType) == "" {
		return errors.New("contentType is required")
	}
	if input.SizeBytes <= 0 {
		return errors.New("sizeBytes must be greater than 0")
	}
	return nil
}

func validateCreatePrescription(input domain.CreatePrescriptionInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if strings.TrimSpace(input.PetID) == "" {
		return errors.New("petId is required")
	}
	if len(input.Medications) == 0 {
		return errors.New("medications is required")
	}
	for _, medication := range input.Medications {
		if strings.TrimSpace(medication.MedicationName) == "" {
			return errors.New("medicationName is required")
		}
	}
	return nil
}

func validateCreateClinicalNote(input domain.CreateClinicalNoteInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if strings.TrimSpace(input.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(input.ReasonForVisit) == "" &&
		strings.TrimSpace(input.Subjective) == "" &&
		strings.TrimSpace(input.Objective) == "" &&
		strings.TrimSpace(input.Assessment) == "" &&
		strings.TrimSpace(input.Plan) == "" {
		return errors.New("at least one clinical note field is required")
	}
	return nil
}

func validateCreateLabOrder(input domain.CreateLabOrderInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if strings.TrimSpace(input.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(input.TestType) == "" {
		return errors.New("testType is required")
	}
	return nil
}

func validateUploadLabResult(input domain.UploadLabResultInput) error {
	if strings.TrimSpace(input.ResultNotes) == "" && strings.TrimSpace(input.ReportObjectPath) == "" {
		return errors.New("resultNotes or reportObjectPath is required")
	}
	if input.CompletedAt != nil {
		if _, err := time.Parse(time.RFC3339, *input.CompletedAt); err != nil {
			return errors.New("completedAt must be RFC3339")
		}
	}
	return nil
}

func validateCreateInvoice(input domain.CreateInvoiceInput) error {
	if strings.TrimSpace(input.LocationID) == "" {
		return errors.New("locationId is required")
	}
	if input.Status != "" && input.Status != "draft" && input.Status != "issued" {
		return errors.New("status must be draft or issued")
	}
	if input.DueAt != nil {
		if _, err := time.Parse(time.RFC3339, *input.DueAt); err != nil {
			return errors.New("dueAt must be RFC3339")
		}
	}
	if input.TaxCents < 0 {
		return errors.New("taxCents must be greater than or equal to 0")
	}
	if input.DiscountCents < 0 {
		return errors.New("discountCents must be greater than or equal to 0")
	}
	if len(input.LineItems) == 0 {
		return errors.New("lineItems is required")
	}
	var subtotal int64
	for _, line := range input.LineItems {
		if strings.TrimSpace(line.Description) == "" {
			return errors.New("line item description is required")
		}
		if line.Quantity <= 0 {
			return errors.New("line item quantity must be greater than 0")
		}
		if line.UnitAmountCents < 0 {
			return errors.New("line item unitAmountCents must be greater than or equal to 0")
		}
		subtotal += int64(line.Quantity) * line.UnitAmountCents
	}
	if subtotal+input.TaxCents-input.DiscountCents < 0 {
		return errors.New("discountCents cannot exceed subtotal plus taxCents")
	}
	return nil
}

func validateCreateStaff(input domain.CreateStaffInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(input.Email) == "" {
		return errors.New("email is required")
	}
	if !strings.Contains(input.Email, "@") {
		return errors.New("email must be valid")
	}
	if !validStaffRole(input.Role) {
		return errors.New("role is not supported for staff management")
	}
	return nil
}

func validateCreateTenant(input domain.CreateTenantInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if input.DefaultCancellationCutoffHours < 0 {
		return errors.New("defaultCancellationCutoffHours must be greater than or equal to 0")
	}
	if err := validateCreateClinicLocation(input.FirstLocation); err != nil {
		return err
	}
	if strings.TrimSpace(input.FirstAdmin.Name) == "" {
		return errors.New("firstAdmin.name is required")
	}
	if strings.TrimSpace(input.FirstAdmin.Email) == "" || !strings.Contains(input.FirstAdmin.Email, "@") {
		return errors.New("firstAdmin.email must be valid")
	}
	return nil
}

func validateUpdateTenant(input domain.UpdateTenantInput) error {
	if input.Name != "" && strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if input.Status != "" && !validTenantStatus(input.Status) {
		return errors.New("status must be active, suspended, or archived")
	}
	if input.DefaultCancellationCutoffHours != nil && *input.DefaultCancellationCutoffHours < 0 {
		return errors.New("defaultCancellationCutoffHours must be greater than or equal to 0")
	}
	return nil
}

func validateCreateClinicLocation(input domain.CreateClinicLocationInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(input.Timezone) == "" {
		return errors.New("timezone is required")
	}
	if input.Email != "" && !strings.Contains(input.Email, "@") {
		return errors.New("email must be valid")
	}
	if input.CancellationCutoffHours != nil && *input.CancellationCutoffHours < 0 {
		return errors.New("cancellationCutoffHours must be greater than or equal to 0")
	}
	return nil
}

func validateUpdateClinicLocation(input domain.UpdateClinicLocationInput) error {
	if input.Name != "" && strings.TrimSpace(input.Name) == "" {
		return errors.New("name is required")
	}
	if input.Timezone != "" && strings.TrimSpace(input.Timezone) == "" {
		return errors.New("timezone is required")
	}
	if input.Email != "" && !strings.Contains(input.Email, "@") {
		return errors.New("email must be valid")
	}
	if input.CancellationCutoffHours != nil && *input.CancellationCutoffHours < 0 {
		return errors.New("cancellationCutoffHours must be greater than or equal to 0")
	}
	if input.Status != "" && !validTenantStatus(input.Status) {
		return errors.New("status must be active, suspended, or archived")
	}
	return nil
}

func validAppointmentType(value domain.AppointmentType) bool {
	for _, item := range domain.PawItProductSpec().SupportedAppointmentTypes {
		if item == value {
			return true
		}
	}
	return false
}

func validSpecies(value domain.Species) bool {
	for _, item := range domain.PawItProductSpec().SupportedSpecies {
		if item == value {
			return true
		}
	}
	return false
}

func validLabStatus(value domain.LabOrderStatus) bool {
	for _, item := range domain.PawItProductSpec().SupportedLabStatuses {
		if item == value {
			return true
		}
	}
	return false
}

func validStaffRole(value domain.Role) bool {
	switch value {
	case domain.RoleClinicAdmin, domain.RoleVeterinarian, domain.RoleReceptionist, domain.RoleVetTechnician, domain.RoleLabTechnician:
		return true
	default:
		return false
	}
}

func validTenantStatus(value string) bool {
	switch value {
	case "active", "suspended", "archived":
		return true
	default:
		return false
	}
}

func roleCan(role string, permissions ...domain.Permission) bool {
	for _, policy := range domain.PawItRolePolicies() {
		if string(policy.Role) != role {
			continue
		}
		for _, granted := range policy.Permissions {
			for _, required := range permissions {
				if granted == required {
					return true
				}
			}
		}
	}
	return false
}

func idempotencyKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("Idempotency-Key"))
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
