package domain

type Store interface {
	Navigation() []NavSection
	Summary() []Metric
	Appointments() []Appointment
	Calendar() map[string]any
	Queue() []QueueEntry
	Patients() []PatientRecord
	PrescriptionTemplates() []PrescriptionTemplate
	ClinicalNotes() []ClinicalNote
	LabTests() []LabTest
	Billing() map[string]any
	Analytics() Analytics
	Feedback() map[string]any
	Doctors() []Person
	Staff() []Person
}

type DemoStore struct{}

func NewDemoStore() DemoStore {
	return DemoStore{}
}

func (DemoStore) Navigation() []NavSection {
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
			{Label: "Billing", Path: "/hospital/billing", Icon: "indian-rupee"},
			{Label: "Analytics", Path: "/hospital/analytics", Icon: "bar-chart-3"},
		}},
		{Label: "Financial", Items: []NavItem{{Label: "Feedback", Path: "/hospital/feedback", Icon: "star"}}},
		{Label: "Reports & Analytics", Items: []NavItem{
			{Label: "Staff Management", Path: "/hospital/staff", Icon: "user-cog"},
			{Label: "Veterinarian Management", Path: "/hospital/doctors", Icon: "clock"},
		}},
	}
}

func (DemoStore) Summary() []Metric {
	return []Metric{
		{Label: "Total Pets", Value: "19", Delta: "+4 this month", Tone: "blue"},
		{Label: "Appointments", Value: "5", Delta: "+100% vs last month", Tone: "green"},
		{Label: "Revenue", Value: "₹0.00", Delta: "From paid invoices", Tone: "green"},
		{Label: "Open Lab Tests", Value: "0", Delta: "No pending diagnostics", Tone: "purple"},
	}
}

func (DemoStore) Appointments() []Appointment {
	return []Appointment{
		{ID: "apt_001", PetName: "Milo", OwnerName: "Priya Sharma", VetName: "Dr. Asha Rao", Time: "09:30", Type: "Vaccination", Status: "confirmed", Contact: "+919876543210"},
		{ID: "apt_002", PetName: "Bruno", OwnerName: "Rajesh Kumar", VetName: "Dr. Vikram Sen", Time: "11:00", Type: "Check-up", Status: "pending", Contact: "+917123456789"},
	}
}

func (DemoStore) Calendar() map[string]any {
	return map[string]any{
		"date": "2026-05-12",
		"statusCounts": map[string]int{
			"scheduled":  2,
			"waiting":    0,
			"inProgress": 0,
			"done":       0,
		},
		"items": DemoStore{}.Appointments(),
	}
}

func (DemoStore) Queue() []QueueEntry {
	return []QueueEntry{
		{ID: "queue_001", PetName: "Coco", OwnerName: "Meera Iyer", Species: "Dog", Priority: "normal", Status: "waiting", WaitMins: 8},
	}
}

func (DemoStore) Patients() []PatientRecord {
	return []PatientRecord{
		{ID: "pet_001", PetName: "Milo", OwnerName: "Priya Sharma", Species: "Cat", Breed: "Indie", Age: "3y", Sex: "Male", Phone: "+919876543210", LastVisit: "2026-05-01", VaccinesDue: 1, OpenPlans: 0},
		{ID: "pet_002", PetName: "Bruno", OwnerName: "Rajesh Kumar", Species: "Dog", Breed: "Labrador", Age: "5y", Sex: "Male", Phone: "+917123456789", LastVisit: "No visits", VaccinesDue: 0, OpenPlans: 1},
	}
}

func (DemoStore) PrescriptionTemplates() []PrescriptionTemplate {
	return []PrescriptionTemplate{
		{ID: "rx_001", Name: "Canine Dermatitis", Condition: "Skin irritation", Category: "Dermatology", Medications: []string{"Chlorhexidine shampoo", "Cetirizine - weight based"}, Instructions: "Avoid self-medication. Recheck if itching persists beyond 5 days."},
		{ID: "rx_002", Name: "Deworming", Condition: "Parasite prevention", Category: "Preventive Care", Medications: []string{"Praziquantel/Pyrantel - weight based"}, Instructions: "Dose by current body weight. Repeat as advised by veterinarian."},
	}
}

func (DemoStore) ClinicalNotes() []ClinicalNote {
	return []ClinicalNote{
		{ID: "note_001", PetName: "Milo", OwnerName: "Priya Sharma", Subject: "Annual wellness exam", Status: "signed", UpdatedAt: "2026-05-01T10:30:00Z"},
	}
}

func (DemoStore) LabTests() []LabTest {
	return []LabTest{}
}

func (DemoStore) Billing() map[string]any {
	return map[string]any{
		"metrics": []Metric{
			{Label: "Total Revenue Today", Value: "₹0.00", Delta: "No payments today", Tone: "green"},
			{Label: "Pending Payments", Value: "₹0.00", Delta: "0 invoices", Tone: "orange"},
			{Label: "Total Revenue All Time", Value: "₹0.00", Delta: "0 paid invoices", Tone: "green"},
			{Label: "Overdue Reminders", Value: "0", Delta: "Bills pending >30 days", Tone: "orange"},
		},
		"invoices": []Invoice{},
	}
}

func (DemoStore) Analytics() Analytics {
	return Analytics{
		Metrics: []Metric{
			{Label: "Total Pets Month", Value: "19", Delta: "Active pet patients", Tone: "blue"},
			{Label: "Average Rating", Value: "0.0", Delta: "0 reviews", Tone: "yellow"},
			{Label: "Total Revenue", Value: "₹0.00", Delta: "From paid invoices", Tone: "green"},
			{Label: "Appointments", Value: "5", Delta: "+100% vs last month", Tone: "purple"},
		},
		SpeciesDistribution: map[string]int{"dogs": 50, "cats": 35, "birds": 10, "other": 5},
		AppointmentStatus:   map[string]int{"confirmed": 1, "pending": 0, "cancelled": 4},
		RevenueTrend:        map[string]string{"Mon": "₹3,037", "Tue": "₹1,079", "Wed": "₹222", "Thu": "₹3,394"},
		CommonDiagnoses:     []Metric{{Label: "Unspecified", Value: "10 pets"}},
	}
}

func (DemoStore) Feedback() map[string]any {
	return map[string]any{
		"metrics": []Metric{
			{Label: "Average Rating", Value: "0"},
			{Label: "Total Reviews", Value: "0"},
			{Label: "Satisfaction Rate", Value: "0%"},
			{Label: "5-Star Reviews", Value: "0"},
		},
		"distribution": map[int]int{5: 0, 4: 0, 3: 0, 2: 0, 1: 0},
		"items":        []Feedback{},
	}
}

func (DemoStore) Doctors() []Person {
	return []Person{
		{ID: "vet_001", Name: "Dr. Asha Rao", Role: "Veterinarian", Specialty: "Small Animal Medicine", Email: "asha.rao@pawit.care", Status: "active"},
		{ID: "vet_002", Name: "Dr. Vikram Sen", Role: "Veterinarian", Specialty: "Surgery", Email: "vikram.sen@pawit.care", Status: "active"},
		{ID: "vet_003", Name: "Dr. Neha Menon", Role: "Veterinarian", Specialty: "Dermatology", Email: "neha.menon@pawit.care", Status: "active"},
	}
}

func (DemoStore) Staff() []Person {
	return []Person{
		{ID: "staff_001", Name: "Teja", Role: "Admin", Email: "teja@pawit.care", Status: "active"},
		{ID: "staff_002", Name: "Chai P", Role: "Receptionist", Email: "chai@pawit.care", Status: "active"},
		{ID: "staff_003", Name: "Anika", Role: "Vet Technician", Email: "anika@pawit.care", Status: "active"},
	}
}
