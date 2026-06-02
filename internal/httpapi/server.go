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
	s.mux.HandleFunc("GET /api/v1/me", s.me)
	s.mux.HandleFunc("GET /api/v1/product-spec", s.productSpec)
	s.mux.HandleFunc("GET /api/v1/role-policies", s.rolePolicies)
	s.mux.HandleFunc("GET /api/v1/navigation", s.navigation)
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
	s.mux.HandleFunc("POST /api/v1/pets/{id}/documents", s.uploadPetDocument)
	s.mux.HandleFunc("GET /api/v1/patients", s.patients)
	s.mux.HandleFunc("GET /api/v1/prescription-templates", s.prescriptionTemplates)
	s.mux.HandleFunc("GET /api/v1/clinical-notes", s.clinicalNotes)
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
	s.mux.HandleFunc("/", s.notFound)
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
	items, err := s.store.Appointments(r.Context(), auth.TenantID)
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
	payload, err := s.store.Calendar(r.Context(), auth.TenantID)
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
	items, err := s.store.Patients(r.Context(), auth.TenantID)
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

func (s *Server) prescriptionTemplates(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.PrescriptionTemplates(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) clinicalNotes(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.ClinicalNotes(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
}

func (s *Server) labTests(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.LabTests(r.Context(), auth.TenantID)
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
	payload, err := s.store.Billing(r.Context(), auth.TenantID)
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
