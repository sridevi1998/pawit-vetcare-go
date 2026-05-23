package httpapi

import (
	"encoding/json"
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
	s.mux.HandleFunc("GET /api/v1/navigation", s.navigation)
	s.mux.HandleFunc("GET /api/v1/dashboard/summary", s.summary)
	s.mux.HandleFunc("GET /api/v1/appointments", s.appointments)
	s.mux.HandleFunc("GET /api/v1/calendar", s.calendar)
	s.mux.HandleFunc("GET /api/v1/queue", s.queue)
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
	writeJSON(w, http.StatusOK, map[string]any{"sections": s.store.Navigation()})
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"metrics": s.store.Summary()})
}

func (s *Server) appointments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Appointments()})
}

func (s *Server) calendar(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Calendar())
}

func (s *Server) queue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Queue()})
}

func (s *Server) patients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Patients()})
}

func (s *Server) prescriptionTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.PrescriptionTemplates()})
}

func (s *Server) clinicalNotes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ClinicalNotes()})
}

func (s *Server) labTests(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.LabTests()})
}

func (s *Server) billing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Billing())
}

func (s *Server) analytics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Analytics())
}

func (s *Server) feedback(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.Feedback())
}

func (s *Server) doctors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Doctors()})
}

func (s *Server) staff(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.Staff()})
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "The requested PawIt endpoint does not exist.")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
