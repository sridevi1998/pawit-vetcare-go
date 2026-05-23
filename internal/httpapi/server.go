package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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
	s.mux.HandleFunc("GET /api/v1/calendar", s.calendar)
	s.mux.HandleFunc("GET /api/v1/queue", s.queue)
	s.mux.HandleFunc("GET /api/v1/pets", s.patients)
	s.mux.HandleFunc("GET /api/v1/patients", s.patients)
	s.mux.HandleFunc("GET /api/v1/prescription-templates", s.prescriptionTemplates)
	s.mux.HandleFunc("GET /api/v1/clinical-notes", s.clinicalNotes)
	s.mux.HandleFunc("GET /api/v1/lab-tests", s.labTests)
	s.mux.HandleFunc("GET /api/v1/billing", s.billing)
	s.mux.HandleFunc("GET /api/v1/analytics", s.analytics)
	s.mux.HandleFunc("GET /api/v1/feedback", s.feedback)
	s.mux.HandleFunc("GET /api/v1/doctors", s.doctors)
	s.mux.HandleFunc("GET /api/v1/staff", s.staff)
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

func (s *Server) patients(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	items, err := s.store.Patients(r.Context(), auth.TenantID)
	writeData(w, map[string]any{"items": items}, err)
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

func (s *Server) billing(w http.ResponseWriter, r *http.Request) {
	auth := authFromContext(r.Context())
	payload, err := s.store.Billing(r.Context(), auth.TenantID)
	writeData(w, payload, err)
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

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
