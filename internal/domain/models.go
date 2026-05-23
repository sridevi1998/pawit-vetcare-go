package domain

type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Delta string `json:"delta,omitempty"`
	Tone  string `json:"tone,omitempty"`
}

type NavSection struct {
	Label string    `json:"label"`
	Items []NavItem `json:"items"`
}

type NavItem struct {
	Label string `json:"label"`
	Path  string `json:"path"`
	Icon  string `json:"icon"`
}

type Appointment struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	VetName   string `json:"vetName"`
	Time      string `json:"time"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Contact   string `json:"contact"`
}

type QueueEntry struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	Species   string `json:"species"`
	Priority  string `json:"priority"`
	Status    string `json:"status"`
	WaitMins  int    `json:"waitMins"`
}

type PatientRecord struct {
	ID          string `json:"id"`
	PetName     string `json:"petName"`
	OwnerName   string `json:"ownerName"`
	Species     string `json:"species"`
	Breed       string `json:"breed"`
	Age         string `json:"age"`
	Sex         string `json:"sex"`
	Phone       string `json:"phone"`
	LastVisit   string `json:"lastVisit"`
	VaccinesDue int    `json:"vaccinesDue"`
	OpenPlans   int    `json:"openPlans"`
}

type PrescriptionTemplate struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Condition    string   `json:"condition"`
	Category     string   `json:"category"`
	Medications  []string `json:"medications"`
	Instructions string   `json:"instructions"`
}

type ClinicalNote struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type LabTest struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	TestType  string `json:"testType"`
	LabCenter string `json:"labCenter"`
	Status    string `json:"status"`
	ReportURL string `json:"reportUrl,omitempty"`
}

type Invoice struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	DueDate   string `json:"dueDate"`
}

type Person struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Specialty string `json:"specialty,omitempty"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}

type Feedback struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"createdAt"`
}

type Analytics struct {
	Metrics             []Metric          `json:"metrics"`
	SpeciesDistribution map[string]int    `json:"speciesDistribution"`
	AppointmentStatus   map[string]int    `json:"appointmentStatus"`
	RevenueTrend        map[string]string `json:"revenueTrend"`
	CommonDiagnoses     []Metric          `json:"commonDiagnoses"`
}
