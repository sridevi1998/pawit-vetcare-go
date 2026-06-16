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

type ClinicLocation struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	Status   string `json:"status"`
}

type Tenant struct {
	ID                             string           `json:"id"`
	Name                           string           `json:"name"`
	LegalName                      string           `json:"legalName,omitempty"`
	Status                         string           `json:"status"`
	StripeCustomerID               string           `json:"stripeCustomerId,omitempty"`
	DefaultCancellationCutoffHours int              `json:"defaultCancellationCutoffHours,omitempty"`
	Locations                      []ClinicLocation `json:"locations"`
	CreatedAt                      string           `json:"createdAt"`
	UpdatedAt                      string           `json:"updatedAt,omitempty"`
}

type CreateTenantInput struct {
	Name                           string                    `json:"name"`
	LegalName                      string                    `json:"legalName,omitempty"`
	DefaultCancellationCutoffHours int                       `json:"defaultCancellationCutoffHours,omitempty"`
	FirstLocation                  CreateClinicLocationInput `json:"firstLocation"`
	FirstAdmin                     FirstClinicAdminInput     `json:"firstAdmin"`
}

type UpdateTenantInput struct {
	Name                           string `json:"name,omitempty"`
	LegalName                      string `json:"legalName,omitempty"`
	Status                         string `json:"status,omitempty"`
	DefaultCancellationCutoffHours *int   `json:"defaultCancellationCutoffHours,omitempty"`
}

type FirstClinicAdminInput struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	Phone             string `json:"phone,omitempty"`
	TemporaryPassword string `json:"temporaryPassword,omitempty"`
}

type CreateClinicLocationInput struct {
	Name                    string `json:"name"`
	Timezone                string `json:"timezone"`
	Phone                   string `json:"phone,omitempty"`
	Email                   string `json:"email,omitempty"`
	CancellationCutoffHours *int   `json:"cancellationCutoffHours,omitempty"`
}

type UpdateClinicLocationInput struct {
	Name                    string `json:"name,omitempty"`
	Timezone                string `json:"timezone,omitempty"`
	Phone                   string `json:"phone,omitempty"`
	Email                   string `json:"email,omitempty"`
	CancellationCutoffHours *int   `json:"cancellationCutoffHours,omitempty"`
	Status                  string `json:"status,omitempty"`
}

type TenantMutationResult struct {
	Tenant     Tenant `json:"tenant"`
	Idempotent bool   `json:"idempotent,omitempty"`
}

type ClinicLocationMutationResult struct {
	Location   ClinicLocation `json:"location"`
	Idempotent bool           `json:"idempotent,omitempty"`
}

type Appointment struct {
	ID                      string            `json:"id"`
	PetName                 string            `json:"petName"`
	OwnerName               string            `json:"ownerName"`
	PrimaryVeterinarian     string            `json:"primaryVeterinarian"`
	AdditionalVeterinarians []string          `json:"additionalVeterinarians"`
	Time                    string            `json:"time"`
	Type                    AppointmentType   `json:"type"`
	Status                  AppointmentStatus `json:"status"`
	Contact                 string            `json:"contact"`
	MeetingURL              string            `json:"meetingUrl,omitempty"`
	Reason                  string            `json:"reason"`
}

type CreateAppointmentInput struct {
	LocationID                string          `json:"locationId"`
	PetID                     string          `json:"petId"`
	Type                      AppointmentType `json:"type"`
	Reason                    string          `json:"reason"`
	StartsAt                  *string         `json:"startsAt,omitempty"`
	EndsAt                    *string         `json:"endsAt,omitempty"`
	PrimaryVeterinarianID     string          `json:"primaryVeterinarianId,omitempty"`
	AdditionalVeterinarianIDs []string        `json:"additionalVeterinarianIds,omitempty"`
	MeetingURL                string          `json:"meetingUrl,omitempty"`
	RequestedByPetParent      bool            `json:"requestedByPetParent,omitempty"`
}

type CancelAppointmentInput struct {
	Reason        string `json:"reason"`
	StaffOverride bool   `json:"staffOverride,omitempty"`
}

type AppointmentMutationResult struct {
	Appointment Appointment `json:"appointment"`
	Idempotent  bool        `json:"idempotent,omitempty"`
}

type QueueEntry struct {
	ID        string      `json:"id"`
	PetName   string      `json:"petName"`
	OwnerName string      `json:"ownerName"`
	Species   string      `json:"species"`
	Priority  string      `json:"priority"`
	Status    QueueStatus `json:"status"`
	WaitMins  int         `json:"waitMins"`
}

type RegisterWalkInInput struct {
	LocationID string `json:"locationId"`
	PetID      string `json:"petId"`
	Priority   string `json:"priority,omitempty"`
	Reason     string `json:"reason"`
}

type UpdateQueueInput struct {
	Reason string `json:"reason,omitempty"`
}

type QueueMutationResult struct {
	QueueEntry QueueEntry `json:"queueEntry"`
	Idempotent bool       `json:"idempotent,omitempty"`
}

type PatientRecord struct {
	ID             string `json:"id"`
	PetName        string `json:"petName"`
	OwnerName      string `json:"ownerName"`
	Species        string `json:"species"`
	Breed          string `json:"breed"`
	Age            string `json:"age"`
	Sex            string `json:"sex"`
	Phone          string `json:"phone"`
	LastVisit      string `json:"lastVisit"`
	VaccinesDue    int    `json:"vaccinesDue"`
	OpenPlans      int    `json:"openPlans"`
	GuardianCount  int    `json:"guardianCount"`
	DocumentsCount int    `json:"documentsCount"`
}

type CreatePetInput struct {
	LocationID      string  `json:"locationId"`
	Name            string  `json:"name"`
	Species         Species `json:"species"`
	Breed           string  `json:"breed,omitempty"`
	Sex             string  `json:"sex,omitempty"`
	EstimatedAge    string  `json:"estimatedAge,omitempty"`
	GuardianName    string  `json:"guardianName"`
	GuardianEmail   string  `json:"guardianEmail,omitempty"`
	Relationship    string  `json:"relationship,omitempty"`
	PrimaryGuardian bool    `json:"primaryGuardian,omitempty"`
}

type ArchivePetInput struct {
	Reason string `json:"reason"`
}

type PetMutationResult struct {
	Pet        PatientRecord `json:"pet"`
	Idempotent bool          `json:"idempotent,omitempty"`
}

type PetDocument struct {
	ID           string `json:"id"`
	PetID        string `json:"petId"`
	Title        string `json:"title"`
	DocumentType string `json:"documentType"`
	ObjectPath   string `json:"objectPath"`
	ContentType  string `json:"contentType"`
	SizeBytes    int64  `json:"sizeBytes"`
	Status       string `json:"status"`
	CreatedAt    string `json:"createdAt"`
}

type PreparePetDocumentUploadInput struct {
	Title        string `json:"title"`
	DocumentType string `json:"documentType"`
	ContentType  string `json:"contentType"`
	SizeBytes    int64  `json:"sizeBytes"`
}

type PetDocumentUploadURLResult struct {
	ObjectPath   string            `json:"objectPath"`
	UploadURL    string            `json:"uploadUrl"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	ExpiresAt    string            `json:"expiresAt"`
	MaxSizeBytes int64             `json:"maxSizeBytes"`
	Idempotent   bool              `json:"idempotent,omitempty"`
}

type PetDocumentDownloadURLResult struct {
	DocumentID  string `json:"documentId"`
	ObjectPath  string `json:"objectPath"`
	DownloadURL string `json:"downloadUrl"`
	Method      string `json:"method"`
	ExpiresAt   string `json:"expiresAt"`
	Idempotent  bool   `json:"idempotent,omitempty"`
}

type UploadPetDocumentInput struct {
	Title        string `json:"title"`
	DocumentType string `json:"documentType"`
	ObjectPath   string `json:"objectPath"`
	ContentType  string `json:"contentType"`
	SizeBytes    int64  `json:"sizeBytes"`
}

type ArchivePetDocumentInput struct {
	Reason string `json:"reason"`
}

type PetDocumentMutationResult struct {
	Document   PetDocument `json:"document"`
	Idempotent bool        `json:"idempotent,omitempty"`
}

type PrescriptionTemplate struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Condition    string   `json:"condition"`
	Category     string   `json:"category"`
	Medications  []string `json:"medications"`
	Instructions string   `json:"instructions"`
}

type Prescription struct {
	ID                  string   `json:"id"`
	PetName             string   `json:"petName"`
	OwnerName           string   `json:"ownerName"`
	Status              string   `json:"status"`
	MedicationNames     []string `json:"medicationNames"`
	Instructions        string   `json:"instructions,omitempty"`
	SharedWithPetParent bool     `json:"sharedWithPetParent"`
	UpdatedAt           string   `json:"updatedAt"`
}

type PrescriptionMedicationInput struct {
	MedicationName string `json:"medicationName"`
	Strength       string `json:"strength,omitempty"`
	Dosage         string `json:"dosage,omitempty"`
	Frequency      string `json:"frequency,omitempty"`
	Duration       string `json:"duration,omitempty"`
	Route          string `json:"route,omitempty"`
	Instructions   string `json:"instructions,omitempty"`
}

type CreatePrescriptionInput struct {
	LocationID                string                        `json:"locationId"`
	PetID                     string                        `json:"petId"`
	AppointmentID             string                        `json:"appointmentId,omitempty"`
	PrescribingVeterinarianID string                        `json:"prescribingVeterinarianId,omitempty"`
	Instructions              string                        `json:"instructions,omitempty"`
	SharedWithPetParent       bool                          `json:"sharedWithPetParent,omitempty"`
	Medications               []PrescriptionMedicationInput `json:"medications"`
}

type FinalizePrescriptionInput struct {
	ShareWithPetParent bool `json:"shareWithPetParent,omitempty"`
}

type PrescriptionMutationResult struct {
	Prescription Prescription `json:"prescription"`
	Idempotent   bool         `json:"idempotent,omitempty"`
}

type ClinicalNote struct {
	ID                  string `json:"id"`
	PetName             string `json:"petName"`
	OwnerName           string `json:"ownerName"`
	Subject             string `json:"subject"`
	Status              string `json:"status"`
	UpdatedAt           string `json:"updatedAt"`
	SharedWithPetParent bool   `json:"sharedWithPetParent"`
}

type CreateClinicalNoteInput struct {
	LocationID          string         `json:"locationId"`
	PetID               string         `json:"petId"`
	AppointmentID       string         `json:"appointmentId,omitempty"`
	ReasonForVisit      string         `json:"reasonForVisit,omitempty"`
	Subjective          string         `json:"subjective,omitempty"`
	Objective           string         `json:"objective,omitempty"`
	Assessment          string         `json:"assessment,omitempty"`
	Plan                string         `json:"plan,omitempty"`
	Vitals              map[string]any `json:"vitals,omitempty"`
	SharedWithPetParent bool           `json:"sharedWithPetParent,omitempty"`
}

type FinalizeClinicalNoteInput struct {
	ShareWithPetParent bool `json:"shareWithPetParent,omitempty"`
}

type ClinicalNoteMutationResult struct {
	ClinicalNote ClinicalNote `json:"clinicalNote"`
	Idempotent   bool         `json:"idempotent,omitempty"`
}

type LabTest struct {
	ID                  string         `json:"id"`
	PetName             string         `json:"petName"`
	OwnerName           string         `json:"ownerName"`
	TestType            string         `json:"testType"`
	LabCenter           string         `json:"labCenter"`
	LabType             string         `json:"labType"`
	Status              LabOrderStatus `json:"status"`
	ReportURL           string         `json:"reportUrl,omitempty"`
	SharedWithPetParent bool           `json:"sharedWithPetParent"`
}

type CreateLabOrderInput struct {
	LocationID    string `json:"locationId"`
	PetID         string `json:"petId"`
	AppointmentID string `json:"appointmentId,omitempty"`
	LabCenterID   string `json:"labCenterId,omitempty"`
	TestType      string `json:"testType"`
	SampleType    string `json:"sampleType,omitempty"`
	Priority      string `json:"priority,omitempty"`
}

type UpdateLabOrderStatusInput struct {
	Status LabOrderStatus `json:"status"`
	Reason string         `json:"reason,omitempty"`
}

type UploadLabResultInput struct {
	ResultNotes        string  `json:"resultNotes,omitempty"`
	ReportObjectPath   string  `json:"reportObjectPath,omitempty"`
	ShareWithPetParent bool    `json:"shareWithPetParent,omitempty"`
	CompletedAt        *string `json:"completedAt,omitempty"`
	MarkOrderCompleted bool    `json:"markOrderCompleted,omitempty"`
}

type LabOrderMutationResult struct {
	LabTest    LabTest `json:"labTest"`
	Idempotent bool    `json:"idempotent,omitempty"`
}

type Invoice struct {
	ID        string `json:"id"`
	PetName   string `json:"petName"`
	OwnerName string `json:"ownerName"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	DueDate   string `json:"dueDate"`
}

type InvoiceLineItemInput struct {
	Description         string `json:"description"`
	Quantity            int    `json:"quantity"`
	UnitAmountCents     int64  `json:"unitAmountCents"`
	RelatedResourceType string `json:"relatedResourceType,omitempty"`
	RelatedResourceID   string `json:"relatedResourceId,omitempty"`
}

type CreateInvoiceInput struct {
	LocationID    string                 `json:"locationId"`
	PetID         string                 `json:"petId,omitempty"`
	Status        string                 `json:"status,omitempty"`
	DueAt         *string                `json:"dueAt,omitempty"`
	TaxCents      int64                  `json:"taxCents,omitempty"`
	DiscountCents int64                  `json:"discountCents,omitempty"`
	LineItems     []InvoiceLineItemInput `json:"lineItems"`
}

type VoidInvoiceInput struct {
	Reason string `json:"reason"`
}

type InvoiceMutationResult struct {
	Invoice    Invoice `json:"invoice"`
	Idempotent bool    `json:"idempotent,omitempty"`
}

type Person struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Specialty string `json:"specialty,omitempty"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}

type CreateStaffInput struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	Role              Role   `json:"role"`
	DefaultLocationID string `json:"defaultLocationId,omitempty"`
}

type StaffMutationResult struct {
	StaffMember Person `json:"staffMember"`
	Idempotent  bool   `json:"idempotent,omitempty"`
}

type LoginInput struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantID   string `json:"tenantId,omitempty"`
	HospitalID string `json:"hospitalId,omitempty"`
	Role       Role   `json:"role,omitempty"`
}

type AuthIdentity struct {
	UserID      string `json:"userId"`
	TenantID    string `json:"tenantId"`
	Role        Role   `json:"role"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type AuthSession struct {
	UserID      string `json:"userId"`
	TenantID    string `json:"tenantId"`
	Role        Role   `json:"role"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Token       string `json:"token"`
	ExpiresAt   string `json:"expiresAt"`
}

type AuditLogEntry struct {
	ID           string `json:"id"`
	ActorUserID  string `json:"actorUserId,omitempty"`
	ActorRole    string `json:"actorRole,omitempty"`
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	Reason       string `json:"reason,omitempty"`
	CreatedAt    string `json:"createdAt"`
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
