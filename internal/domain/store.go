package domain

import "context"

type Store interface {
	Ready(ctx context.Context) error
	ProductSpec(ctx context.Context) (ProductSpec, error)
	RolePolicies(ctx context.Context) ([]RolePolicy, error)
	Navigation(ctx context.Context, tenantID string) ([]NavSection, error)
	Summary(ctx context.Context, tenantID string) ([]Metric, error)
	Appointments(ctx context.Context, tenantID string) ([]Appointment, error)
	CreateAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error)
	CancelAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, appointmentID string, input CancelAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error)
	Calendar(ctx context.Context, tenantID string) (map[string]any, error)
	Queue(ctx context.Context, tenantID string) ([]QueueEntry, error)
	RegisterWalkIn(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input RegisterWalkInInput, idempotencyKey string) (QueueMutationResult, error)
	UpdateQueueStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, queueID string, status QueueStatus, input UpdateQueueInput, idempotencyKey string) (QueueMutationResult, error)
	Patients(ctx context.Context, tenantID string) ([]PatientRecord, error)
	CreatePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePetInput, idempotencyKey string) (PetMutationResult, error)
	ArchivePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input ArchivePetInput, idempotencyKey string) (PetMutationResult, error)
	PetDocuments(ctx context.Context, tenantID string, petID string) ([]PetDocument, error)
	UploadPetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input UploadPetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error)
	ArchivePetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, documentID string, input ArchivePetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error)
	Prescriptions(ctx context.Context, tenantID string) ([]Prescription, error)
	PrescriptionTemplates(ctx context.Context, tenantID string) ([]PrescriptionTemplate, error)
	CreatePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error)
	FinalizePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, prescriptionID string, input FinalizePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error)
	ClinicalNotes(ctx context.Context, tenantID string) ([]ClinicalNote, error)
	LabTests(ctx context.Context, tenantID string) ([]LabTest, error)
	CreateLabOrder(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateLabOrderInput, idempotencyKey string) (LabOrderMutationResult, error)
	UpdateLabOrderStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UpdateLabOrderStatusInput, idempotencyKey string) (LabOrderMutationResult, error)
	UploadLabResult(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UploadLabResultInput, idempotencyKey string) (LabOrderMutationResult, error)
	Billing(ctx context.Context, tenantID string) (map[string]any, error)
	CreateInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error)
	VoidInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, invoiceID string, input VoidInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error)
	Analytics(ctx context.Context, tenantID string) (Analytics, error)
	Feedback(ctx context.Context, tenantID string) (map[string]any, error)
	Doctors(ctx context.Context, tenantID string) ([]Person, error)
	Staff(ctx context.Context, tenantID string) ([]Person, error)
	CreateStaff(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateStaffInput, idempotencyKey string) (StaffMutationResult, error)
}

type DemoStore struct{}

func NewDemoStore() DemoStore {
	return DemoStore{}
}

func (DemoStore) Ready(ctx context.Context) error {
	return nil
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

func (DemoStore) Summary(ctx context.Context, tenantID string) ([]Metric, error) {
	return []Metric{
		{Label: "Total Pets", Value: "19", Delta: "+4 this month", Tone: "blue"},
		{Label: "Appointments", Value: "5", Delta: "+100% vs last month", Tone: "green"},
		{Label: "Revenue", Value: "$0.00", Delta: "Stripe-ready invoice model", Tone: "green"},
		{Label: "Open Lab Tests", Value: "0", Delta: "No pending diagnostics", Tone: "purple"},
	}, nil
}

func (DemoStore) Appointments(ctx context.Context, tenantID string) ([]Appointment, error) {
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
	}, nil
}

func (DemoStore) CreateAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error) {
	status := AppointmentScheduled
	if actorRole == RolePetParent || input.RequestedByPetParent {
		status = AppointmentRequested
	}
	timeLabel := "Unscheduled"
	if input.StartsAt != nil && *input.StartsAt != "" {
		timeLabel = *input.StartsAt
	}
	return AppointmentMutationResult{
		Appointment: Appointment{
			ID:                      "apt_demo_created",
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
		},
	}, nil
}

func (DemoStore) CancelAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole Role, appointmentID string, input CancelAppointmentInput, idempotencyKey string) (AppointmentMutationResult, error) {
	return AppointmentMutationResult{
		Appointment: Appointment{
			ID:      appointmentID,
			PetName: "Demo Pet",
			Status:  AppointmentCancelled,
			Reason:  input.Reason,
		},
	}, nil
}

func (DemoStore) Calendar(ctx context.Context, tenantID string) (map[string]any, error) {
	appointments, err := DemoStore{}.Appointments(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"date": "2026-05-12",
		"statusCounts": map[string]int{
			"scheduled":  2,
			"waiting":    0,
			"inProgress": 0,
			"done":       0,
		},
		"items": appointments,
	}, nil
}

func (DemoStore) Queue(ctx context.Context, tenantID string) ([]QueueEntry, error) {
	return []QueueEntry{
		{ID: "queue_001", PetName: "Coco", OwnerName: "Morgan Lee", Species: "Dog", Priority: "normal", Status: QueueWaiting, WaitMins: 8},
	}, nil
}

func (DemoStore) RegisterWalkIn(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input RegisterWalkInInput, idempotencyKey string) (QueueMutationResult, error) {
	priority := input.Priority
	if priority == "" {
		priority = "normal"
	}
	return QueueMutationResult{QueueEntry: QueueEntry{
		ID:        "queue_demo_created",
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Species:   "dog",
		Priority:  priority,
		Status:    QueueWaiting,
		WaitMins:  0,
	}}, nil
}

func (DemoStore) UpdateQueueStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, queueID string, status QueueStatus, input UpdateQueueInput, idempotencyKey string) (QueueMutationResult, error) {
	return QueueMutationResult{QueueEntry: QueueEntry{
		ID:        queueID,
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Species:   "dog",
		Priority:  "normal",
		Status:    status,
		WaitMins:  0,
	}}, nil
}

func (DemoStore) Patients(ctx context.Context, tenantID string) ([]PatientRecord, error) {
	return []PatientRecord{
		{ID: "pet_001", PetName: "Milo", OwnerName: "Avery Parker", Species: string(SpeciesCat), Breed: "Domestic Shorthair", Age: "3y", Sex: "Male", Phone: "+13125550110", LastVisit: "2026-05-01", VaccinesDue: 1, OpenPlans: 0, GuardianCount: 2, DocumentsCount: 3},
		{ID: "pet_002", PetName: "Bruno", OwnerName: "Jordan Ellis", Species: string(SpeciesDog), Breed: "Labrador Retriever", Age: "5y", Sex: "Male", Phone: "+14155550192", LastVisit: "No visits", VaccinesDue: 0, OpenPlans: 1, GuardianCount: 1, DocumentsCount: 1},
	}, nil
}

func (DemoStore) CreatePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePetInput, idempotencyKey string) (PetMutationResult, error) {
	return PetMutationResult{Pet: PatientRecord{
		ID:            "pet_demo_created",
		PetName:       input.Name,
		OwnerName:     input.GuardianName,
		Species:       string(input.Species),
		Breed:         input.Breed,
		Age:           input.EstimatedAge,
		Sex:           input.Sex,
		Phone:         input.GuardianEmail,
		LastVisit:     "No visits",
		GuardianCount: 1,
	}}, nil
}

func (DemoStore) ArchivePet(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input ArchivePetInput, idempotencyKey string) (PetMutationResult, error) {
	return PetMutationResult{Pet: PatientRecord{ID: petID, PetName: "Archived Demo Pet"}}, nil
}

func (DemoStore) PetDocuments(ctx context.Context, tenantID string, petID string) ([]PetDocument, error) {
	return []PetDocument{
		{ID: "doc_001", PetID: petID, Title: "Rabies certificate", DocumentType: "vaccine_history", ObjectPath: "tenant_demo/pets/doc_001/rabies.pdf", ContentType: "application/pdf", SizeBytes: 1024, Status: "active", CreatedAt: "2026-05-12T10:00:00Z"},
	}, nil
}

func (DemoStore) UploadPetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, input UploadPetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error) {
	return PetDocumentMutationResult{Document: PetDocument{
		ID:           "doc_demo_created",
		PetID:        petID,
		Title:        input.Title,
		DocumentType: input.DocumentType,
		ObjectPath:   input.ObjectPath,
		ContentType:  input.ContentType,
		SizeBytes:    input.SizeBytes,
		Status:       "active",
	}}, nil
}

func (DemoStore) ArchivePetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole Role, petID string, documentID string, input ArchivePetDocumentInput, idempotencyKey string) (PetDocumentMutationResult, error) {
	return PetDocumentMutationResult{Document: PetDocument{
		ID:     documentID,
		PetID:  petID,
		Title:  "Archived demo document",
		Status: "archived",
	}}, nil
}

func (DemoStore) PrescriptionTemplates(ctx context.Context, tenantID string) ([]PrescriptionTemplate, error) {
	return []PrescriptionTemplate{
		{ID: "rx_001", Name: "Canine Dermatitis", Condition: "Skin irritation", Category: "Dermatology", Medications: []string{"Chlorhexidine shampoo", "Cetirizine - weight based"}, Instructions: "Avoid self-medication. Recheck if itching persists beyond 5 days."},
		{ID: "rx_002", Name: "Deworming", Condition: "Parasite prevention", Category: "Preventive Care", Medications: []string{"Praziquantel/Pyrantel - weight based"}, Instructions: "Dose by current body weight. Repeat as advised by veterinarian."},
	}, nil
}

func (DemoStore) Prescriptions(ctx context.Context, tenantID string) ([]Prescription, error) {
	return []Prescription{
		{ID: "rx_draft_001", PetName: "Bruno", OwnerName: "Jordan Ellis", Status: "draft", MedicationNames: []string{"Cetirizine"}, Instructions: "Draft pending veterinarian review.", SharedWithPetParent: false, UpdatedAt: "2026-05-12T14:00:00Z"},
	}, nil
}

func (DemoStore) CreatePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreatePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error) {
	medications := make([]string, 0, len(input.Medications))
	for _, medication := range input.Medications {
		medications = append(medications, medication.MedicationName)
	}
	return PrescriptionMutationResult{Prescription: Prescription{
		ID:                  "rx_demo_created",
		PetName:             "Demo Pet",
		OwnerName:           "Demo Guardian",
		Status:              "draft",
		MedicationNames:     medications,
		Instructions:        input.Instructions,
		SharedWithPetParent: input.SharedWithPetParent,
	}}, nil
}

func (DemoStore) FinalizePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole Role, prescriptionID string, input FinalizePrescriptionInput, idempotencyKey string) (PrescriptionMutationResult, error) {
	return PrescriptionMutationResult{Prescription: Prescription{
		ID:                  prescriptionID,
		PetName:             "Demo Pet",
		OwnerName:           "Demo Guardian",
		Status:              "finalized",
		MedicationNames:     []string{"Demo medication"},
		SharedWithPetParent: input.ShareWithPetParent,
	}}, nil
}

func (DemoStore) ClinicalNotes(ctx context.Context, tenantID string) ([]ClinicalNote, error) {
	return []ClinicalNote{
		{ID: "note_001", PetName: "Milo", OwnerName: "Avery Parker", Subject: "Annual wellness exam", Status: "finalized", UpdatedAt: "2026-05-01T10:30:00Z", SharedWithPetParent: true},
	}, nil
}

func (DemoStore) LabTests(ctx context.Context, tenantID string) ([]LabTest, error) {
	return []LabTest{
		{ID: "lab_001", PetName: "Bruno", OwnerName: "Jordan Ellis", TestType: "Skin scraping", LabCenter: "Northside Veterinary Lab", LabType: "external", Status: LabSentOut, SharedWithPetParent: false},
	}, nil
}

func (DemoStore) CreateLabOrder(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateLabOrderInput, idempotencyKey string) (LabOrderMutationResult, error) {
	return LabOrderMutationResult{LabTest: LabTest{
		ID:        "lab_demo_created",
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		TestType:  input.TestType,
		LabCenter: "Internal lab",
		LabType:   "internal",
		Status:    LabOrdered,
	}}, nil
}

func (DemoStore) UpdateLabOrderStatus(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UpdateLabOrderStatusInput, idempotencyKey string) (LabOrderMutationResult, error) {
	return LabOrderMutationResult{LabTest: LabTest{
		ID:        labOrderID,
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		TestType:  "Demo lab test",
		LabCenter: "Internal lab",
		LabType:   "internal",
		Status:    input.Status,
	}}, nil
}

func (DemoStore) UploadLabResult(ctx context.Context, tenantID string, actorUserID string, actorRole Role, labOrderID string, input UploadLabResultInput, idempotencyKey string) (LabOrderMutationResult, error) {
	status := LabInProgress
	if input.MarkOrderCompleted {
		status = LabCompleted
	}
	return LabOrderMutationResult{LabTest: LabTest{
		ID:                  labOrderID,
		PetName:             "Demo Pet",
		OwnerName:           "Demo Guardian",
		TestType:            "Demo lab test",
		LabCenter:           "Internal lab",
		LabType:             "internal",
		Status:              status,
		ReportURL:           input.ReportObjectPath,
		SharedWithPetParent: input.ShareWithPetParent,
	}}, nil
}

func (DemoStore) Billing(ctx context.Context, tenantID string) (map[string]any, error) {
	return map[string]any{
		"metrics": []Metric{
			{Label: "Total Revenue Today", Value: "$0.00", Delta: "No payments today", Tone: "green"},
			{Label: "Pending Payments", Value: "$0.00", Delta: "0 invoices", Tone: "orange"},
			{Label: "Total Revenue All Time", Value: "$0.00", Delta: "0 paid invoices", Tone: "green"},
			{Label: "Overdue Reminders", Value: "0", Delta: "Bills pending >30 days", Tone: "orange"},
		},
		"invoices": []Invoice{},
	}, nil
}

func (DemoStore) CreateInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error) {
	total := input.TaxCents - input.DiscountCents
	for _, line := range input.LineItems {
		total += int64(line.Quantity) * line.UnitAmountCents
	}
	status := input.Status
	if status == "" {
		status = "issued"
	}
	return InvoiceMutationResult{Invoice: Invoice{
		ID:        "inv_demo_created",
		PetName:   "Demo Pet",
		OwnerName: "Demo Guardian",
		Amount:    total,
		Status:    status,
		DueDate:   "",
	}}, nil
}

func (DemoStore) VoidInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole Role, invoiceID string, input VoidInvoiceInput, idempotencyKey string) (InvoiceMutationResult, error) {
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

func (DemoStore) Staff(ctx context.Context, tenantID string) ([]Person, error) {
	return []Person{
		{ID: "staff_001", Name: "Teja", Role: string(RoleClinicAdmin), Email: "teja@pawit.care", Status: "active"},
		{ID: "staff_002", Name: "Chai P", Role: string(RoleReceptionist), Email: "chai@pawit.care", Status: "active"},
		{ID: "staff_003", Name: "Anika", Role: string(RoleVetTechnician), Email: "anika@pawit.care", Status: "active"},
	}, nil
}

func (DemoStore) CreateStaff(ctx context.Context, tenantID string, actorUserID string, actorRole Role, input CreateStaffInput, idempotencyKey string) (StaffMutationResult, error) {
	return StaffMutationResult{StaffMember: Person{
		ID:     "staff_demo_created",
		Name:   input.Name,
		Role:   string(input.Role),
		Email:  input.Email,
		Status: "invited",
	}}, nil
}
