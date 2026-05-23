package domain

type Role string

const (
	RoleSuperAdmin    Role = "SuperAdmin"
	RoleClinicAdmin   Role = "ClinicAdmin"
	RoleVeterinarian  Role = "Veterinarian"
	RoleReceptionist  Role = "Receptionist"
	RoleVetTechnician Role = "VetTechnician"
	RoleLabTechnician Role = "LabTechnician"
	RolePetParent     Role = "PetParent"
)

type AppointmentType string

const (
	AppointmentInClinic     AppointmentType = "in_clinic"
	AppointmentTelemedicine AppointmentType = "telemedicine"
	AppointmentWalkIn       AppointmentType = "walk_in"
	AppointmentFollowUp     AppointmentType = "follow_up"
	AppointmentVaccination  AppointmentType = "vaccination"
	AppointmentLab          AppointmentType = "lab_diagnostic"
	AppointmentProcedure    AppointmentType = "procedure_consult"
)

type AppointmentStatus string

const (
	AppointmentRequested       AppointmentStatus = "requested"
	AppointmentScheduled       AppointmentStatus = "scheduled"
	AppointmentConfirmed       AppointmentStatus = "confirmed"
	AppointmentCheckedIn       AppointmentStatus = "checked_in"
	AppointmentWaiting         AppointmentStatus = "waiting"
	AppointmentInProgress      AppointmentStatus = "in_progress"
	AppointmentCompleted       AppointmentStatus = "completed"
	AppointmentCancelled       AppointmentStatus = "cancelled"
	AppointmentNoShow          AppointmentStatus = "no_show"
	AppointmentNeedsReschedule AppointmentStatus = "needs_reschedule"
	AppointmentRejected        AppointmentStatus = "rejected"
)

type LabOrderStatus string

const (
	LabOrdered         LabOrderStatus = "ordered"
	LabSampleCollected LabOrderStatus = "sample_collected"
	LabSentOut         LabOrderStatus = "sent_out"
	LabInProgress      LabOrderStatus = "in_progress"
	LabCompleted       LabOrderStatus = "completed"
	LabCancelled       LabOrderStatus = "cancelled"
)

type Species string

const (
	SpeciesDog Species = "dog"
	SpeciesCat Species = "cat"
)

type Permission string

const (
	PermissionTenantManage               Permission = "tenant.manage"
	PermissionLocationManage             Permission = "location.manage"
	PermissionStaffManage                Permission = "staff.manage"
	PermissionPetRecordManage            Permission = "pet_record.manage"
	PermissionPetDocumentUpload          Permission = "pet_document.upload"
	PermissionAppointmentManage          Permission = "appointment.manage"
	PermissionAppointmentRequestOwn      Permission = "appointment_request.own"
	PermissionQueueManage                Permission = "queue.manage"
	PermissionClinicalNoteView           Permission = "clinical_note.view"
	PermissionClinicalNoteDraft          Permission = "clinical_note.draft"
	PermissionClinicalNoteFinalize       Permission = "clinical_note.finalize"
	PermissionClinicalNoteViewShared     Permission = "clinical_note.view_shared"
	PermissionPrescriptionView           Permission = "prescription.view"
	PermissionPrescriptionDraft          Permission = "prescription.draft"
	PermissionPrescriptionFinalize       Permission = "prescription.finalize"
	PermissionPrescriptionViewShared     Permission = "prescription.view_shared"
	PermissionLabOrderCreate             Permission = "lab_order.create"
	PermissionLabOrderProcess            Permission = "lab_order.process"
	PermissionLabResultShare             Permission = "lab_result.share"
	PermissionLabResultViewShared        Permission = "lab_result.view_shared"
	PermissionInvoiceCreate              Permission = "invoice.create"
	PermissionInvoiceManage              Permission = "invoice.manage"
	PermissionInvoicePayOwn              Permission = "invoice.pay_own"
	PermissionPaymentRefundVoid          Permission = "payment.refund_void"
	PermissionAnalyticsFull              Permission = "analytics.full"
	PermissionAnalyticsOperational       Permission = "analytics.operational"
	PermissionAnalyticsClinical          Permission = "analytics.clinical"
	PermissionAnalyticsLab               Permission = "analytics.lab"
	PermissionDataExport                 Permission = "data.export"
	PermissionSupportImpersonation       Permission = "support.impersonation"
	PermissionAuditLogView               Permission = "audit_log.view"
	PermissionSharedMedicalRecordViewOwn Permission = "shared_medical_record.view_own"
	PermissionPetParentProfileManageOwn  Permission = "pet_parent_profile.manage_own"
	PermissionPetRecordManageOwn         Permission = "pet_record.manage_own"
)

type ProductSpec struct {
	ProductName                  string              `json:"productName"`
	ShortName                    string              `json:"shortName"`
	SupportedSpecies             []Species           `json:"supportedSpecies"`
	SupportedAppointmentTypes    []AppointmentType   `json:"supportedAppointmentTypes"`
	SupportedAppointmentStatuses []AppointmentStatus `json:"supportedAppointmentStatuses"`
	SupportedLabStatuses         []LabOrderStatus    `json:"supportedLabStatuses"`
	PaymentProvider              string              `json:"paymentProvider"`
	Currency                     string              `json:"currency"`
	PetParentAuth                string              `json:"petParentAuth"`
	Telemedicine                 TelemedicineSpec    `json:"telemedicine"`
	Labs                         LabSpec             `json:"labs"`
	CancellationPolicy           CancellationPolicy  `json:"cancellationPolicy"`
	RecordDeletionPolicy         string              `json:"recordDeletionPolicy"`
	AIAdvisoryEnabled            bool                `json:"aiAdvisoryEnabled"`
	GroomingEnabled              bool                `json:"groomingEnabled"`
	HIPAAAligned                 bool                `json:"hipaaAligned"`
}

type TelemedicineSpec struct {
	Mode                     string `json:"mode"`
	BuiltInVideoRoomsEnabled bool   `json:"builtInVideoRoomsEnabled"`
}

type LabSpec struct {
	InternalLabsEnabled          bool   `json:"internalLabsEnabled"`
	ExternalLabsEnabled          bool   `json:"externalLabsEnabled"`
	ExternalIntegrationMode      string `json:"externalIntegrationMode"`
	ThirdPartyAPIIntegrationInV1 bool   `json:"thirdPartyApiIntegrationInV1"`
}

type CancellationPolicy struct {
	DefaultCutoffHours   int  `json:"defaultCutoffHours"`
	TenantConfigurable   bool `json:"tenantConfigurable"`
	LocationOverride     bool `json:"locationOverride"`
	StaffOverrideAllowed bool `json:"staffOverrideAllowed"`
}

type RolePolicy struct {
	Role         Role         `json:"role"`
	Description  string       `json:"description"`
	Permissions  []Permission `json:"permissions"`
	Restrictions []string     `json:"restrictions,omitempty"`
}

func PawItProductSpec() ProductSpec {
	return ProductSpec{
		ProductName: "PawIt VetCare",
		ShortName:   "PawIt",
		SupportedSpecies: []Species{
			SpeciesDog,
			SpeciesCat,
		},
		SupportedAppointmentTypes: []AppointmentType{
			AppointmentInClinic,
			AppointmentTelemedicine,
			AppointmentWalkIn,
			AppointmentFollowUp,
			AppointmentVaccination,
			AppointmentLab,
			AppointmentProcedure,
		},
		SupportedAppointmentStatuses: []AppointmentStatus{
			AppointmentRequested,
			AppointmentScheduled,
			AppointmentConfirmed,
			AppointmentCheckedIn,
			AppointmentWaiting,
			AppointmentInProgress,
			AppointmentCompleted,
			AppointmentCancelled,
			AppointmentNoShow,
			AppointmentNeedsReschedule,
			AppointmentRejected,
		},
		SupportedLabStatuses: []LabOrderStatus{
			LabOrdered,
			LabSampleCollected,
			LabSentOut,
			LabInProgress,
			LabCompleted,
			LabCancelled,
		},
		PaymentProvider:      "stripe",
		Currency:             "USD",
		PetParentAuth:        "email_password",
		Telemedicine:         TelemedicineSpec{Mode: "manual_meeting_link", BuiltInVideoRoomsEnabled: false},
		Labs:                 LabSpec{InternalLabsEnabled: true, ExternalLabsEnabled: true, ExternalIntegrationMode: "manual_status_based", ThirdPartyAPIIntegrationInV1: false},
		CancellationPolicy:   CancellationPolicy{DefaultCutoffHours: 24, TenantConfigurable: true, LocationOverride: true, StaffOverrideAllowed: true},
		RecordDeletionPolicy: "archive_cancel_void_only",
		AIAdvisoryEnabled:    false,
		GroomingEnabled:      false,
		HIPAAAligned:         true,
	}
}

func PawItRolePolicies() []RolePolicy {
	return []RolePolicy{
		{
			Role:        RoleSuperAdmin,
			Description: "Internal PawIt platform administrator with audited support access.",
			Permissions: []Permission{
				PermissionTenantManage,
				PermissionSupportImpersonation,
				PermissionAuditLogView,
			},
			Restrictions: []string{"Tenant clinical data requires audited support/impersonation flow."},
		},
		{
			Role:        RoleClinicAdmin,
			Description: "Tenant administrator for clinic settings, staff, records, exports, and billing settings.",
			Permissions: []Permission{
				PermissionLocationManage,
				PermissionStaffManage,
				PermissionPetRecordManage,
				PermissionPetDocumentUpload,
				PermissionAppointmentManage,
				PermissionQueueManage,
				PermissionClinicalNoteView,
				PermissionPrescriptionView,
				PermissionLabOrderCreate,
				PermissionLabOrderProcess,
				PermissionInvoiceCreate,
				PermissionInvoiceManage,
				PermissionPaymentRefundVoid,
				PermissionAnalyticsFull,
				PermissionDataExport,
				PermissionAuditLogView,
			},
			Restrictions: []string{"Hard-delete is not allowed for clinical, financial, identity, or audit records."},
		},
		{
			Role:        RoleVeterinarian,
			Description: "Clinical care provider responsible for clinical notes, prescriptions, lab orders, and shared records.",
			Permissions: []Permission{
				PermissionPetRecordManage,
				PermissionAppointmentManage,
				PermissionQueueManage,
				PermissionClinicalNoteView,
				PermissionClinicalNoteDraft,
				PermissionClinicalNoteFinalize,
				PermissionPrescriptionView,
				PermissionPrescriptionDraft,
				PermissionPrescriptionFinalize,
				PermissionLabOrderCreate,
				PermissionLabResultShare,
				PermissionAnalyticsClinical,
			},
		},
		{
			Role:        RoleReceptionist,
			Description: "Front-desk operator for appointment, queue, document, and billing workflows.",
			Permissions: []Permission{
				PermissionPetRecordManage,
				PermissionPetDocumentUpload,
				PermissionAppointmentManage,
				PermissionQueueManage,
				PermissionClinicalNoteView,
				PermissionPrescriptionView,
				PermissionLabOrderCreate,
				PermissionInvoiceCreate,
				PermissionAnalyticsOperational,
			},
			Restrictions: []string{"Cannot finalize clinical notes or prescriptions.", "Refunds/voids require ClinicAdmin permission."},
		},
		{
			Role:        RoleVetTechnician,
			Description: "Clinical support user for vitals, draft notes, prescription drafts, and delegated lab workflows.",
			Permissions: []Permission{
				PermissionPetRecordManage,
				PermissionAppointmentManage,
				PermissionClinicalNoteView,
				PermissionClinicalNoteDraft,
				PermissionPrescriptionView,
				PermissionPrescriptionDraft,
				PermissionLabOrderCreate,
				PermissionLabOrderProcess,
				PermissionAnalyticsClinical,
			},
			Restrictions: []string{"Veterinarian finalization is required for notes and prescriptions."},
		},
		{
			Role:        RoleLabTechnician,
			Description: "Lab workflow user for sample status, result upload, and lab completion.",
			Permissions: []Permission{
				PermissionLabOrderProcess,
				PermissionAnalyticsLab,
			},
			Restrictions: []string{"No full invoice, payment method, discount, or financial history access.", "Cannot create clinical lab orders by default."},
		},
		{
			Role:        RolePetParent,
			Description: "External pet guardian user for self-service records, requests, documents, and payments.",
			Permissions: []Permission{
				PermissionPetParentProfileManageOwn,
				PermissionPetRecordManageOwn,
				PermissionPetDocumentUpload,
				PermissionAppointmentRequestOwn,
				PermissionClinicalNoteViewShared,
				PermissionPrescriptionViewShared,
				PermissionLabResultViewShared,
				PermissionInvoicePayOwn,
				PermissionSharedMedicalRecordViewOwn,
			},
			Restrictions: []string{"Can only view shared medical records.", "Cannot cancel appointments inside clinic cutoff window."},
		},
	}
}
