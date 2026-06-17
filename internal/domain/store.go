package domain

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Store interface {
	Ready(ctx context.Context) error
	Authenticate(ctx context.Context, input LoginInput) (AuthIdentity, error)
	ProductSpec(ctx context.Context) (ProductSpec, error)
	RolePolicies(ctx context.Context) ([]RolePolicy, error)
	Navigation(ctx context.Context, tenantID string) ([]NavSection, error)
	Locations(ctx context.Context, tenantID string) ([]ClinicLocation, error)
	Tenants(ctx context.Context) ([]Tenant, error)
	Tenant(ctx context.Context, tenantID string) (Tenant, error)
	CreateTenant(ctx context.Context, actorTenantID string, actorUserID string, actorRole Role, input CreateTenantInput, idempotencyKey string) (TenantMutationResult, error)
	UpdateTenant(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input UpdateTenantInput, idempotencyKey string) (TenantMutationResult, error)
	CreateTenantLocation(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateClinicLocationInput, idempotencyKey string) (ClinicLocationMutationResult, error)
	UpdateTenantLocation(ctx context.Context, tenantID string, locationID string, actorUserID string, actorRole Role, input UpdateClinicLocationInput, idempotencyKey string) (ClinicLocationMutationResult, error)
	Summary(ctx context.Context, tenantID string) ([]Metric, error)
	Appointments(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]Appointment, error)
	CreateAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error)
	CancelAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, appointmentID string, input CancelAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error)
	Calendar(ctx context.Context, tenantID string, actorUserID string, actorRole Role) (map[string]any, error)
	Queue(ctx context.Context, tenantID string) ([]QueueEntry, error)
	RegisterWalkIn(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input RegisterWalkInInput, idempotencyKey string) (QueueMutationResult, error)
	UpdateQueueStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, queueID string, status QueueStatus, input UpdateQueueInput, idempotencyKey string) (QueueMutationResult, error)
	Patients(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]PatientRecord, error)
	CreatePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePetInput, idempotencyKey string) (PetMutationResult, error)
	ArchivePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input ArchivePetInput, idempotencyKey string) (PetMutationResult, error)
	PetDocuments(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string) ([]PetDocument, error)
	PreparePetDocumentUpload(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input PreparePetDocumentUploadInput, idempotencyKey string) (PetDocumentUploadURLResult, error)
	UploadPetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input UploadPetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error)
	CreatePetDocumentDownload(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, documentID string, idempotencyKey string) (PetDocumentDownloadURLResult, error)
	ArchivePetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, documentID string, input ArchivePetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error)
	Prescriptions(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]Prescription, error)
	PrescriptionTemplates(ctx context.Context, tenantID string) ([]PrescriptionTemplate, error)
	CreatePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error)
	FinalizePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, prescriptionID string, input FinalizePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error)
	ClinicalNotes(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]ClinicalNote, error)
	CreateClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateClinicalNoteInput, idempotencyKey string) (ClinicalNoteMutationResult, error)
	FinalizeClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole Role, clinicalNoteID string, input FinalizeClinicalNoteInput, idempotencyKey string) (ClinicalNoteMutationResult, error)
	LabTests(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]LabTest, error)
	CreateLabOrder(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateLabOrderInput, idempotencyKey string) (LabOrderMutationResult, error)
	UpdateLabOrderStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UpdateLabOrderStatusInput, idempotencyKey string) (LabOrderMutationResult, error)
	UploadLabResult(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UploadLabResultInput, idempotencyKey string) (LabOrderMutationResult, error)
	Billing(ctx context.Context, tenantID string, actorUserID string, actorRole Role) (map[string]any, error)
	CreateInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error)
	VoidInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, invoiceID string, input VoidInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error)
	Analytics(ctx context.Context, tenantID string) (Analytics, error)
	Feedback(ctx context.Context, tenantID string) (map[string]any, error)
	Doctors(ctx context.Context, tenantID string) ([]Person, error)
	Staff(ctx context.Context, tenantID string) ([]Person, error)
	CreateStaff(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateStaffInput, idempotencyKey string) (StaffMutationResult, error)
	AuditLogs(ctx context.Context, tenantID string) ([]AuditLogEntry, error)
}

type DemoStore struct {
	mu            sync.Mutex
	seeded        bool
	tenants       []Tenant
	appointments  []Appointment
	queue         []QueueEntry
	patients      []PatientRecord
	petDocuments  map[string][]PetDocument
	prescriptions []Prescription
	clinicalNotes []ClinicalNote
	labTests      []LabTest
	invoices      []Invoice
	staff         []Person
}

func NewDemoStore() *DemoStore {
	store := &DemoStore{}
	store.seedLocked()
	return store
}

func (s *DemoStore) seedLocked() {
	if s.seeded {
		return
	}
	s.tenants = []Tenant{demoTenant()}
	s.appointments = demoAppointments()
	s.queue = demoQueue()
	s.patients = demoPatients()
	s.petDocuments = demoPetDocuments()
	s.prescriptions = demoPrescriptions()
	s.clinicalNotes = demoClinicalNotes()
	s.labTests = demoLabTests()
	s.invoices = []Invoice{}
	s.staff = demoStaff()
	s.seeded = true
}

func (DemoStore) Ready(ctx context.Context) error {
	return nil
}

func (DemoStore) Authenticate(ctx context.Context, input LoginInput) (AuthIdentity, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	if email == "" || password == "" {
		return AuthIdentity{}, ErrInvalidCredentials
	}

	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID == "" {
		tenantID = demoTenantForHospital(input.HospitalID)
	}
	role := input.Role
	if role == "" {
		role = RoleClinicAdmin
	}

	demoUsers := map[string]struct {
		userID      string
		displayName string
		roles       []Role
	}{
		"superadmin@pawit.example": {userID: "user_demo_super_admin", displayName: "Sam PawIt Admin", roles: []Role{RoleSuperAdmin}},
		"admin@pawit.example":      {userID: "user_demo_admin", displayName: "Asha Clinic Admin", roles: []Role{RoleClinicAdmin}},
		"doctor@pawit.example":     {userID: "user_demo_doctor", displayName: "Dr. Asha Rao", roles: []Role{RoleVeterinarian}},
		"frontdesk@pawit.example":  {userID: "user_demo_reception", displayName: "Riya Front Desk", roles: []Role{RoleReceptionist}},
		"tech@pawit.example":       {userID: "user_demo_tech", displayName: "Nina Vet Technician", roles: []Role{RoleVetTechnician}},
		"lab@pawit.example":        {userID: "user_demo_lab", displayName: "Leo Lab Technician", roles: []Role{RoleLabTechnician}},
		"parent@pawit.example":     {userID: "user_demo_parent", displayName: "Avery Parker", roles: []Role{RolePetParent}},
	}
	user, ok := demoUsers[email]
	if !ok || password != "pawit-demo" {
		return AuthIdentity{}, ErrInvalidCredentials
	}
	if !roleIn(role, user.roles) {
		return AuthIdentity{}, ErrInvalidCredentials
	}

	return AuthIdentity{UserID: user.userID, TenantID: tenantID, Role: role, DisplayName: user.displayName, Email: email}, nil
}

func demoTenantForHospital(hospitalID string) string {
	switch strings.ToUpper(strings.TrimSpace(hospitalID)) {
	case "", "HOSP-001":
		return "tenant_demo_clinic"
	default:
		return strings.TrimSpace(hospitalID)
	}
}

func (DemoStore) ProductSpec(ctx context.Context) (ProductSpec, error) {
	return PawItProductSpec(), nil
}

func (DemoStore) RolePolicies(ctx context.Context) ([]RolePolicy, error) {
	return PawItRolePolicies(), nil
}

func (DemoStore) Navigation(ctx context.Context, tenantID string) ([]NavSection, error) {
	return []NavSection{
		{Label: "Main", Items: []NavItem{{Label: "Appointments", Path: "/hospital/appointments", Icon: "calendar-days"}}},
		{Label: "Patient Management", Items: []NavItem{
			{Label: "Calendar", Path: "/hospital/calendar", Icon: "calendar"},
			{Label: "Patient Queue", Path: "/hospital/queue", Icon: "users"},
			{Label: "Pet Records", Path: "/hospital/patients", Icon: "file-text"},
			{Label: "Prescriptions", Path: "/hospital/prescriptions", Icon: "clipboard"},
		}},
		{Label: "Clinical", Items: []NavItem{
			{Label: "Clinical Notes", Path: "/hospital/consultations", Icon: "message-square"},
			{Label: "Lab & Diagnostics", Path: "/hospital/lab-tests", Icon: "flask-conical"},
			{Label: "Billing", Path: "/hospital/billing", Icon: "dollar-sign"},
			{Label: "Analytics", Path: "/hospital/analytics", Icon: "bar-chart-3"},
		}},
		{Label: "Financial", Items: []NavItem{{Label: "Feedback", Path: "/hospital/feedback", Icon: "star"}}},
		{Label: "Reports & Analytics", Items: []NavItem{
			{Label: "Staff Management", Path: "/hospital/staff", Icon: "user-cog"},
			{Label: "Veterinarian Management", Path: "/hospital/doctors", Icon: "clock"},
		}},
	}, nil
}

func (s *DemoStore) Locations(ctx context.Context, tenantID string) ([]ClinicLocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for _, tenant := range s.tenants {
		if tenant.ID == tenantID {
			return append([]ClinicLocation(nil), tenant.Locations...), nil
		}
	}
	return append([]ClinicLocation(nil), demoTenant().Locations...), nil
}

func (s *DemoStore) Tenants(ctx context.Context) ([]Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]Tenant(nil), s.tenants...), nil
}

func (s *DemoStore) Tenant(ctx context.Context, tenantID string) (Tenant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for _, tenant := range s.tenants {
		if tenant.ID == tenantID {
			return tenant, nil
		}
	}
	return Tenant{}, ErrNotFound
}

func (s *DemoStore) CreateTenant(ctx context.Context, actorTenantID string, actorUserID string, actorRole Role, input CreateTenantInput, idempotencyKey string) (TenantMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	tenant := demoTenant()
	tenant.ID = "tenant_demo_created_" + strconv.Itoa(len(s.tenants)+1)
	tenant.Name = input.Name
	tenant.LegalName = input.LegalName
	tenant.DefaultCancellationCutoffHours = input.DefaultCancellationCutoffHours
	tenant.Locations = []ClinicLocation{{
		ID:       "loc_demo_created_1",
		Name:     input.FirstLocation.Name,
		Timezone: input.FirstLocation.Timezone,
		Phone:    input.FirstLocation.Phone,
		Email:    input.FirstLocation.Email,
		Status:   "active",
	}}
	s.tenants = append([]Tenant{tenant}, s.tenants...)
	return TenantMutationResult{Tenant: tenant}, nil
}

func (s *DemoStore) UpdateTenant(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input UpdateTenantInput, idempotencyKey string) (TenantMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.tenants {
		if s.tenants[i].ID != tenantID {
			continue
		}
		if input.Name != "" {
			s.tenants[i].Name = input.Name
		}
		if input.LegalName != "" {
			s.tenants[i].LegalName = input.LegalName
		}
		if input.Status != "" {
			s.tenants[i].Status = input.Status
		}
		if input.DefaultCancellationCutoffHours != nil {
			s.tenants[i].DefaultCancellationCutoffHours = *input.DefaultCancellationCutoffHours
		}
		return TenantMutationResult{Tenant: s.tenants[i]}, nil
	}
	tenant := demoTenant()
	tenant.ID = tenantID
	if input.Name != "" {
		tenant.Name = input.Name
	}
	if input.LegalName != "" {
		tenant.LegalName = input.LegalName
	}
	if input.Status != "" {
		tenant.Status = input.Status
	}
	return TenantMutationResult{Tenant: tenant}, nil
}

func (s *DemoStore) CreateTenantLocation(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateClinicLocationInput, idempotencyKey string) (ClinicLocationMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	location := ClinicLocation{
		ID:       "loc_demo_created_" + strconv.Itoa(len(s.tenants)+1),
		Name:     input.Name,
		Timezone: input.Timezone,
		Phone:    input.Phone,
		Email:    input.Email,
		Status:   "active",
	}
	for i := range s.tenants {
		if s.tenants[i].ID == tenantID {
			location.ID = "loc_demo_created_" + strconv.Itoa(len(s.tenants[i].Locations)+1)
			s.tenants[i].Locations = append(s.tenants[i].Locations, location)
			return ClinicLocationMutationResult{Location: location}, nil
		}
	}
	return ClinicLocationMutationResult{Location: location}, nil
}

func (DemoStore) UpdateTenantLocation(ctx context.Context, tenantID string, locationID string, actorUserID string, actorRole Role, input UpdateClinicLocationInput, idempotencyKey string) (ClinicLocationMutationResult, error) {
	location := ClinicLocation{ID: locationID, Name: "PawIt Demo Clinic", Timezone: "America/Chicago", Status: "active"}
	if input.Name != "" {
		location.Name = input.Name
	}
	if input.Timezone != "" {
		location.Timezone = input.Timezone
	}
	if input.Status != "" {
		location.Status = input.Status
	}
	location.Phone = input.Phone
	location.Email = input.Email
	return ClinicLocationMutationResult{Location: location}, nil
}

func demoTenant() Tenant {
	return Tenant{
		ID:                             "tenant_demo_clinic",
		Name:                           "PawIt Demo Clinic",
		Status:                         "active",
		DefaultCancellationCutoffHours: 24,
		Locations: []ClinicLocation{{
			ID:       "loc_demo_main",
			Name:     "PawIt Demo Clinic",
			Timezone: "America/Chicago",
			Phone:    "+13125550100",
			Email:    "hello@pawit.example",
			Status:   "active",
		}},
		CreatedAt: "2026-01-01T00:00:00Z",
	}
}

func demoAppointments() []Appointment {
	return []Appointment{
		{
			ID: "apt_001", PetName: "Milo", OwnerName: "Avery Parker",
			PrimaryVeterinarian: "Dr. Asha Rao", AdditionalVeterinarians: []string{"Dr. Vikram Sen"},
			Time: "09:30", Type: AppointmentVaccination, Status: AppointmentConfirmed,
			Contact: "+13125550110", Reason: "Rabies booster and wellness check",
		},
		{
			ID: "apt_002", PetName: "Bruno", OwnerName: "Jordan Ellis",
			PrimaryVeterinarian: "Dr. Vikram Sen", AdditionalVeterinarians: []string{},
			Time: "11:00", Type: AppointmentTelemedicine, Status: AppointmentRequested,
			Contact: "+14155550192", MeetingURL: "https://meet.example.com/pawit-demo", Reason: "Follow-up on skin irritation",
		},
	}
}

func demoQueue() []QueueEntry {
	return []QueueEntry{
		{ID: "queue_001", PetName: "Coco", OwnerName: "Morgan Lee", Species: "Dog", Priority: "normal", Status: QueueWaiting, WaitMins: 8},
	}
}

func demoPatients() []PatientRecord {
	return []PatientRecord{
		{ID: "pet_001", PetName: "Milo", OwnerName: "Avery Parker", Species: string(SpeciesCat), Breed: "Domestic Shorthair", Age: "3y", Sex: "Male", Phone: "+13125550110", LastVisit: "2026-05-01", VaccinesDue: 1, OpenPlans: 0, GuardianCount: 2, DocumentsCount: 3},
		{ID: "pet_002", PetName: "Bruno", OwnerName: "Jordan Ellis", Species: string(SpeciesDog), Breed: "Labrador Retriever", Age: "5y", Sex: "Male", Phone: "+14155550192", LastVisit: "No visits", VaccinesDue: 0, OpenPlans: 1, GuardianCount: 1, DocumentsCount: 1},
	}
}

func demoPetDocuments() map[string][]PetDocument {
	return map[string][]PetDocument{
		"pet_001": {
			{ID: "doc_001", PetID: "pet_001", Title: "Rabies certificate", DocumentType: "vaccine_history", ObjectPath: "tenant_demo/pets/doc_001/rabies.pdf", ContentType: "application/pdf", SizeBytes: 1024, Status: "active", CreatedAt: "2026-05-12T10:00:00Z"},
		},
	}
}

func demoPrescriptions() []Prescription {
	return []Prescription{
		{ID: "rx_draft_001", PetName: "Bruno", OwnerName: "Jordan Ellis", Status: "draft", MedicationNames: []string{"Cetirizine"}, Instructions: "Draft pending veterinarian review.", SharedWithPetParent: false, UpdatedAt: "2026-05-12T14:00:00Z"},
	}
}

func demoClinicalNotes() []ClinicalNote {
	return []ClinicalNote{
		{ID: "note_001", PetName: "Milo", OwnerName: "Avery Parker", Subject: "Annual wellness exam", Status: "finalized", UpdatedAt: "2026-05-01T10:30:00Z", SharedWithPetParent: true},
	}
}

func demoLabTests() []LabTest {
	return []LabTest{
		{ID: "lab_001", PetName: "Bruno", OwnerName: "Jordan Ellis", TestType: "Skin scraping", LabCenter: "Northside Veterinary Lab", LabType: "external", Status: LabSentOut, SharedWithPetParent: false},
	}
}

func demoStaff() []Person {
	return []Person{
		{ID: "staff_001", Name: "Teja", Role: string(RoleClinicAdmin), Email: "teja@pawit.care", Status: "active"},
		{ID: "staff_002", Name: "Chai P", Role: string(RoleReceptionist), Email: "chai@pawit.care", Status: "active"},
		{ID: "staff_003", Name: "Anika", Role: string(RoleVetTechnician), Email: "anika@pawit.care", Status: "active"},
	}
}

func (DemoStore) Summary(ctx context.Context, tenantID string) ([]Metric, error) {
	return []Metric{
		{Label: "Total Pets", Value: "19", Delta: "+4 this month", Tone: "blue"},
		{Label: "Appointments", Value: "5", Delta: "+100% vs last month", Tone: "green"},
		{Label: "Revenue", Value: "$0.00", Delta: "Stripe-ready invoice model", Tone: "green"},
		{Label: "Open Lab Tests", Value: "0", Delta: "No pending diagnostics", Tone: "purple"},
	}, nil
}

func (s *DemoStore) Appointments(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]Appointment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	items := append([]Appointment(nil), s.appointments...)
	if actorRole == RolePetParent {
		if len(items) == 0 {
			return items, nil
		}
		return items[:1], nil
	}
	return items, nil
}

func (s *DemoStore) CreateAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error) {
	status := AppointmentScheduled
	if actorRole == RolePetParent || input.RequestedByPetParent {
		status = AppointmentRequested
	}
	timeLabel := "Unscheduled"
	if input.StartsAt != nil && *input.StartsAt != "" {
		timeLabel = *input.StartsAt
	}
	s.mu.Lock()
	s.seedLocked()
	appointment := Appointment{
		ID:                      "apt_demo_created_" + strconv.Itoa(len(s.appointments)+1),
		PetName:                 "Demo Pet",
		OwnerName:               "Demo Guardian",
		PrimaryVeterinarian:     "Unassigned",
		AdditionalVeterinarians: []string{},
		Time:                    timeLabel,
		Type:                    input.Type,
		Status:                  status,
		Contact:                 "",
		MeetingURL:              input.MeetingURL,
		Reason:                  input.Reason,
	}
	s.appointments = append([]Appointment{appointment}, s.appointments...)
	s.mu.Unlock()
	return AppointmentMutationResult{Appointment: appointment}, nil
}

func (s *DemoStore) CancelAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, appointmentID string, input CancelAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.appointments {
		if s.appointments[i].ID == appointmentID {
			s.appointments[i].Status = AppointmentCancelled
			if strings.TrimSpace(input.Reason) != "" {
				s.appointments[i].Reason = input.Reason
			}
			return AppointmentMutationResult{Appointment: s.appointments[i]}, nil
		}
	}
	return AppointmentMutationResult{
		Appointment: Appointment{
			ID:                      appointmentID,
			PetName:                 "Demo Pet",
			OwnerName:               "Demo Guardian",
			PrimaryVeterinarian:     "Unassigned",
			AdditionalVeterinarians: []string{},
			Time:                    "Unscheduled",
			Type:                    AppointmentInClinic,
			Status:                  AppointmentCancelled,
			Contact:                 "",
			Reason:                  input.Reason,
		},
	}, nil
}

func (s *DemoStore) Calendar(ctx context.Context, tenantID string, actorUserID string, actorRole Role) (map[string]any, error) {
	appointments, err := s.Appointments(ctx, tenantID, actorUserID, actorRole)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{
		"scheduled":  0,
		"waiting":    0,
		"inProgress": 0,
		"done":       0,
	}
	for _, appointment := range appointments {
		switch appointment.Status {
		case AppointmentScheduled, AppointmentConfirmed, AppointmentRequested:
			counts["scheduled"]++
		case AppointmentWaiting:
			counts["waiting"]++
		case AppointmentInProgress:
			counts["inProgress"]++
		case AppointmentCompleted:
			counts["done"]++
		}
	}
	return map[string]any{
		"date":         "2026-05-12",
		"statusCounts": counts,
		"items":        appointments,
	}, nil
}

func (s *DemoStore) Queue(ctx context.Context, tenantID string) ([]QueueEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]QueueEntry(nil), s.queue...), nil
}

func (s *DemoStore) RegisterWalkIn(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input RegisterWalkInInput, idempotencyKey string) (QueueMutationResult, error) {
	priority := input.Priority
	if priority == "" {
		priority = "normal"
	}
	s.mu.Lock()
	s.seedLocked()
	entry := QueueEntry{
		ID:        "queue_demo_created_" + strconv.Itoa(len(s.queue)+1),
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Species:   "dog",
		Priority:  priority,
		Status:    QueueWaiting,
		WaitMins:  0,
	}
	s.queue = append([]QueueEntry{entry}, s.queue...)
	s.mu.Unlock()
	return QueueMutationResult{QueueEntry: entry}, nil
}

func (s *DemoStore) UpdateQueueStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, queueID string, status QueueStatus, input UpdateQueueInput, idempotencyKey string) (QueueMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.queue {
		if s.queue[i].ID == queueID {
			s.queue[i].Status = status
			return QueueMutationResult{QueueEntry: s.queue[i]}, nil
		}
	}
	entry := QueueEntry{
		ID:        queueID,
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Species:   "dog",
		Priority:  "normal",
		Status:    status,
		WaitMins:  0,
	}
	s.queue = append([]QueueEntry{entry}, s.queue...)
	return QueueMutationResult{QueueEntry: entry}, nil
}

func (s *DemoStore) Patients(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]PatientRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]PatientRecord(nil), s.patients...), nil
}

func (s *DemoStore) CreatePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePetInput, idempotencyKey string) (PetMutationResult, error) {
	s.mu.Lock()
	s.seedLocked()
	pet := PatientRecord{
		ID:            "pet_demo_created_" + strconv.Itoa(len(s.patients)+1),
		PetName:       input.Name,
		OwnerName:     input.GuardianName,
		Species:       string(input.Species),
		Breed:         input.Breed,
		Age:           input.EstimatedAge,
		Sex:           input.Sex,
		Phone:         input.GuardianEmail,
		LastVisit:     "No visits",
		GuardianCount: 1,
	}
	s.patients = append([]PatientRecord{pet}, s.patients...)
	s.mu.Unlock()
	return PetMutationResult{Pet: pet}, nil
}

func (s *DemoStore) ArchivePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input ArchivePetInput, idempotencyKey string) (PetMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.patients {
		if s.patients[i].ID == petID {
			archived := s.patients[i]
			s.patients = append(s.patients[:i], s.patients[i+1:]...)
			return PetMutationResult{Pet: archived}, nil
		}
	}
	return PetMutationResult{Pet: PatientRecord{ID: petID, PetName: "Archived Demo Pet"}}, nil
}

func (s *DemoStore) PetDocuments(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string) ([]PetDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]PetDocument(nil), s.petDocuments[petID]...), nil
}

func (DemoStore) PreparePetDocumentUpload(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input PreparePetDocumentUploadInput, idempotencyKey string) (PetDocumentUploadURLResult, error) {
	objectPath := "tenants/" + strings.TrimSpace(tenantID) + "/pets/" + strings.TrimSpace(petID) + "/documents/demo-upload"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	return PetDocumentUploadURLResult{
		ObjectPath: objectPath,
		UploadURL:  "https://storage.googleapis.com/pawit-vetcare-documents-dev/" + objectPath + "?X-Goog-Signature=local-dev",
		Method:     "PUT",
		Headers: map[string]string{
			"Content-Type": strings.TrimSpace(input.ContentType),
		},
		ExpiresAt:    expiresAt.Format(time.RFC3339),
		MaxSizeBytes: 25 * 1024 * 1024,
	}, nil
}

func (s *DemoStore) UploadPetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input UploadPetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	document := PetDocument{
		ID:           "doc_demo_created_" + strconv.Itoa(len(s.petDocuments[petID])+1),
		PetID:        petID,
		Title:        input.Title,
		DocumentType: input.DocumentType,
		ObjectPath:   input.ObjectPath,
		ContentType:  input.ContentType,
		SizeBytes:    input.SizeBytes,
		Status:       "active",
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	s.petDocuments[petID] = append([]PetDocument{document}, s.petDocuments[petID]...)
	return PetDocumentMutationResult{Document: document}, nil
}

func (DemoStore) CreatePetDocumentDownload(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, documentID string, idempotencyKey string) (PetDocumentDownloadURLResult, error) {
	objectPath := "tenant_demo/pets/doc_001/rabies.pdf"
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	return PetDocumentDownloadURLResult{
		DocumentID:  documentID,
		ObjectPath:  objectPath,
		DownloadURL: "https://storage.googleapis.com/pawit-vetcare-documents-dev/" + objectPath + "?X-Goog-Signature=local-dev",
		Method:      "GET",
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *DemoStore) ArchivePetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, documentID string, input ArchivePetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.petDocuments[petID] {
		if s.petDocuments[petID][i].ID == documentID {
			s.petDocuments[petID][i].Status = "archived"
			return PetDocumentMutationResult{Document: s.petDocuments[petID][i]}, nil
		}
	}
	return PetDocumentMutationResult{Document: PetDocument{ID: documentID, PetID: petID, Title: "Archived demo document", Status: "archived"}}, nil
}

func (DemoStore) PrescriptionTemplates(ctx context.Context, tenantID string) ([]PrescriptionTemplate, error) {
	return []PrescriptionTemplate{
		{ID: "rx_001", Name: "Canine Dermatitis", Condition: "Skin irritation", Category: "Dermatology", Medications: []string{"Chlorhexidine shampoo", "Cetirizine - weight based"}, Instructions: "Avoid self-medication. Recheck if itching persists beyond 5 days."},
		{ID: "rx_002", Name: "Deworming", Condition: "Parasite prevention", Category: "Preventive Care", Medications: []string{"Praziquantel/Pyrantel - weight based"}, Instructions: "Dose by current body weight. Repeat as advised by veterinarian."},
	}, nil
}

func (s *DemoStore) Prescriptions(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]Prescription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]Prescription(nil), s.prescriptions...), nil
}

func (s *DemoStore) CreatePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error) {
	medications := make([]string, 0, len(input.Medications))
	for _, medication := range input.Medications {
		medications = append(medications, medication.MedicationName)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	prescription := Prescription{
		ID:                  "rx_demo_created_" + strconv.Itoa(len(s.prescriptions)+1),
		PetName:             "Demo Pet",
		OwnerName:           "Demo Guardian",
		Status:              "draft",
		MedicationNames:     medications,
		Instructions:        input.Instructions,
		SharedWithPetParent: input.SharedWithPetParent,
		UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
	}
	s.prescriptions = append([]Prescription{prescription}, s.prescriptions...)
	return PrescriptionMutationResult{Prescription: prescription}, nil
}

func (s *DemoStore) FinalizePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, prescriptionID string, input FinalizePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.prescriptions {
		if s.prescriptions[i].ID == prescriptionID {
			s.prescriptions[i].Status = "finalized"
			s.prescriptions[i].SharedWithPetParent = input.ShareWithPetParent
			s.prescriptions[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return PrescriptionMutationResult{Prescription: s.prescriptions[i]}, nil
		}
	}
	return PrescriptionMutationResult{Prescription: Prescription{ID: prescriptionID, PetName: "Demo Pet", OwnerName: "Demo Guardian", Status: "finalized", MedicationNames: []string{"Demo medication"}, SharedWithPetParent: input.ShareWithPetParent, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}}, nil
}

func (s *DemoStore) ClinicalNotes(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]ClinicalNote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]ClinicalNote(nil), s.clinicalNotes...), nil
}

func (s *DemoStore) CreateClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateClinicalNoteInput, idempotencyKey string) (ClinicalNoteMutationResult, error) {
	subject := strings.TrimSpace(input.ReasonForVisit)
	if subject == "" {
		subject = strings.TrimSpace(input.Assessment)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	note := ClinicalNote{
		ID:                  "note_demo_created_" + strconv.Itoa(len(s.clinicalNotes)+1),
		PetName:             "Demo Pet",
		OwnerName:           "Demo Guardian",
		Subject:             subject,
		Status:              "draft",
		UpdatedAt:           time.Now().UTC().Format(time.RFC3339),
		SharedWithPetParent: input.SharedWithPetParent,
	}
	s.clinicalNotes = append([]ClinicalNote{note}, s.clinicalNotes...)
	return ClinicalNoteMutationResult{ClinicalNote: note}, nil
}

func (s *DemoStore) FinalizeClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole Role, clinicalNoteID string, input FinalizeClinicalNoteInput, idempotencyKey string) (ClinicalNoteMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.clinicalNotes {
		if s.clinicalNotes[i].ID == clinicalNoteID {
			s.clinicalNotes[i].Status = "finalized"
			s.clinicalNotes[i].SharedWithPetParent = input.ShareWithPetParent
			s.clinicalNotes[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return ClinicalNoteMutationResult{ClinicalNote: s.clinicalNotes[i]}, nil
		}
	}
	return ClinicalNoteMutationResult{ClinicalNote: ClinicalNote{ID: clinicalNoteID, PetName: "Demo Pet", OwnerName: "Demo Guardian", Subject: "Demo finalized clinical note", Status: "finalized", UpdatedAt: time.Now().UTC().Format(time.RFC3339), SharedWithPetParent: input.ShareWithPetParent}}, nil
}

func (s *DemoStore) LabTests(ctx context.Context, tenantID string, actorUserID string, actorRole Role) ([]LabTest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]LabTest(nil), s.labTests...), nil
}

func (s *DemoStore) CreateLabOrder(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateLabOrderInput, idempotencyKey string) (LabOrderMutationResult, error) {
	s.mu.Lock()
	s.seedLocked()
	lab := LabTest{
		ID:        "lab_demo_created_" + strconv.Itoa(len(s.labTests)+1),
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		TestType:  input.TestType,
		LabCenter: "Internal lab",
		LabType:   "internal",
		Status:    LabOrdered,
	}
	s.labTests = append([]LabTest{lab}, s.labTests...)
	s.mu.Unlock()
	return LabOrderMutationResult{LabTest: lab}, nil
}

func (s *DemoStore) UpdateLabOrderStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UpdateLabOrderStatusInput, idempotencyKey string) (LabOrderMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.labTests {
		if s.labTests[i].ID == labOrderID {
			s.labTests[i].Status = input.Status
			return LabOrderMutationResult{LabTest: s.labTests[i]}, nil
		}
	}
	lab := LabTest{
		ID:        labOrderID,
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		TestType:  "Demo lab test",
		LabCenter: "Internal lab",
		LabType:   "internal",
		Status:    input.Status,
	}
	s.labTests = append([]LabTest{lab}, s.labTests...)
	return LabOrderMutationResult{LabTest: lab}, nil
}

func (s *DemoStore) UploadLabResult(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UploadLabResultInput, idempotencyKey string) (LabOrderMutationResult, error) {
	status := LabInProgress
	if input.MarkOrderCompleted {
		status = LabCompleted
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.labTests {
		if s.labTests[i].ID == labOrderID {
			s.labTests[i].Status = status
			s.labTests[i].ReportURL = input.ReportObjectPath
			s.labTests[i].SharedWithPetParent = input.ShareWithPetParent
			return LabOrderMutationResult{LabTest: s.labTests[i]}, nil
		}
	}
	lab := LabTest{
		ID:                  labOrderID,
		PetName:             "Demo Pet",
		OwnerName:           "Demo Guardian",
		TestType:            "Demo lab test",
		LabCenter:           "Internal lab",
		LabType:             "internal",
		Status:              status,
		ReportURL:           input.ReportObjectPath,
		SharedWithPetParent: input.ShareWithPetParent,
	}
	s.labTests = append([]LabTest{lab}, s.labTests...)
	return LabOrderMutationResult{LabTest: lab}, nil
}

func (s *DemoStore) Billing(ctx context.Context, tenantID string, actorUserID string, actorRole Role) (map[string]any, error) {
	s.mu.Lock()
	s.seedLocked()
	invoices := append([]Invoice(nil), s.invoices...)
	s.mu.Unlock()
	return map[string]any{
		"metrics": []Metric{
			{Label: "Total Revenue Today", Value: "$0.00", Delta: "No payments today", Tone: "green"},
			{Label: "Pending Payments", Value: "$0.00", Delta: "0 invoices", Tone: "orange"},
			{Label: "Total Revenue All Time", Value: "$0.00", Delta: "0 paid invoices", Tone: "green"},
			{Label: "Overdue Reminders", Value: "0", Delta: "Bills pending >30 days", Tone: "orange"},
		},
		"invoices": invoices,
	}, nil
}

func (s *DemoStore) CreateInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error) {
	total := input.TaxCents - input.DiscountCents
	for _, line := range input.LineItems {
		total += int64(line.Quantity) * line.UnitAmountCents
	}
	status := input.Status
	if status == "" {
		status = "issued"
	}
	s.mu.Lock()
	s.seedLocked()
	invoice := Invoice{
		ID:        "inv_demo_created_" + strconv.Itoa(len(s.invoices)+1),
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Amount:    total,
		Status:    status,
		DueDate:   "",
	}
	s.invoices = append([]Invoice{invoice}, s.invoices...)
	s.mu.Unlock()
	return InvoiceMutationResult{Invoice: invoice}, nil
}

func (s *DemoStore) VoidInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, invoiceID string, input VoidInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	for i := range s.invoices {
		if s.invoices[i].ID == invoiceID {
			s.invoices[i].Status = "void"
			return InvoiceMutationResult{Invoice: s.invoices[i]}, nil
		}
	}
	return InvoiceMutationResult{Invoice: Invoice{
		ID:        invoiceID,
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Amount:    0,
		Status:    "void",
	}}, nil
}

func (DemoStore) Analytics(ctx context.Context, tenantID string) (Analytics, error) {
	return Analytics{
		Metrics: []Metric{
			{Label: "Total Pets Month", Value: "19", Delta: "Active pet patients", Tone: "blue"},
			{Label: "Average Rating", Value: "0.0", Delta: "0 reviews", Tone: "yellow"},
			{Label: "Total Revenue", Value: "$0.00", Delta: "From paid invoices", Tone: "green"},
			{Label: "Appointments", Value: "5", Delta: "+100% vs last month", Tone: "purple"},
		},
		SpeciesDistribution: map[string]int{"dogs": 55, "cats": 45},
		AppointmentStatus:   map[string]int{"confirmed": 1, "requested": 1, "cancelled": 0},
		RevenueTrend:        map[string]string{"Mon": "$3,037", "Tue": "$1,079", "Wed": "$222", "Thu": "$3,394"},
		CommonDiagnoses:     []Metric{{Label: "Unspecified", Value: "10 pets"}},
	}, nil
}

func (DemoStore) Feedback(ctx context.Context, tenantID string) (map[string]any, error) {
	return map[string]any{
		"metrics": []Metric{
			{Label: "Average Rating", Value: "0"},
			{Label: "Total Reviews", Value: "0"},
			{Label: "Satisfaction Rate", Value: "0%"},
			{Label: "5-Star Reviews", Value: "0"},
		},
		"distribution": map[int]int{5: 0, 4: 0, 3: 0, 2: 0, 1: 0},
		"items":        []Feedback{},
	}, nil
}

func (DemoStore) Doctors(ctx context.Context, tenantID string) ([]Person, error) {
	return []Person{
		{ID: "vet_001", Name: "Dr. Asha Rao", Role: string(RoleVeterinarian), Specialty: "Small Animal Medicine", Email: "asha.rao@pawit.care", Status: "active"},
		{ID: "vet_002", Name: "Dr. Vikram Sen", Role: string(RoleVeterinarian), Specialty: "Surgery", Email: "vikram.sen@pawit.care", Status: "active"},
		{ID: "vet_003", Name: "Dr. Neha Menon", Role: string(RoleVeterinarian), Specialty: "Dermatology", Email: "neha.menon@pawit.care", Status: "active"},
	}, nil
}

func (s *DemoStore) Staff(ctx context.Context, tenantID string) ([]Person, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLocked()
	return append([]Person(nil), s.staff...), nil
}

func (s *DemoStore) CreateStaff(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateStaffInput, idempotencyKey string) (StaffMutationResult, error) {
	s.mu.Lock()
	s.seedLocked()
	person := Person{
		ID:     "staff_demo_created_" + strconv.Itoa(len(s.staff)+1),
		Name:   input.Name,
		Role:   string(input.Role),
		Email:  input.Email,
		Status: "invited",
	}
	s.staff = append([]Person{person}, s.staff...)
	s.mu.Unlock()
	return StaffMutationResult{StaffMember: person}, nil
}

func (DemoStore) AuditLogs(ctx context.Context, tenantID string) ([]AuditLogEntry, error) {
	return []AuditLogEntry{
		{
			ID:           "audit_001",
			ActorUserID:  "user_demo_admin",
			ActorRole:    string(RoleClinicAdmin),
			Action:       "pet_document.upload",
			ResourceType: "pet_document",
			ResourceID:   "doc_001",
			Reason:       "Rabies certificate uploaded",
			CreatedAt:    "2026-05-12T10:00:00Z",
		},
		{
			ID:           "audit_002",
			ActorUserID:  "user_demo_admin",
			ActorRole:    string(RoleClinicAdmin),
			Action:       "invoice.void",
			ResourceType: "invoice",
			ResourceID:   "inv_001",
			Reason:       "Duplicate invoice",
			CreatedAt:    "2026-05-12T11:30:00Z",
		},
	}, nil
}

func roleIn(role Role, roles []Role) bool {
	for _, item := range roles {
		if item == role {
			return true
		}
	}
	return false
}
