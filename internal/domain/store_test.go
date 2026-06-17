package domain

import (
	"context"
	"testing"
)

func TestDemoStoreAppointmentCreateAndCancelPersist(t *testing.T) {
	ctx := context.Background()
	store := NewDemoStore()

	created, err := store.CreateAppointment(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin, CreateAppointmentInput{
		LocationID: "loc_001",
		PetID:      "pet_001",
		Type:       AppointmentInClinic,
		Reason:     "Stateful appointment test",
	}, "appointment-test")
	if err != nil {
		t.Fatalf("create appointment: %v", err)
	}

	items, err := store.Appointments(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin)
	if err != nil {
		t.Fatalf("list appointments: %v", err)
	}
	if len(items) == 0 || items[0].ID != created.Appointment.ID {
		t.Fatalf("expected created appointment first, got %#v", items)
	}

	cancelled, err := store.CancelAppointment(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin, created.Appointment.ID, CancelAppointmentInput{
		Reason: "Stateful cancellation test",
	}, "appointment-cancel-test")
	if err != nil {
		t.Fatalf("cancel appointment: %v", err)
	}
	if cancelled.Appointment.Status != AppointmentCancelled {
		t.Fatalf("expected cancelled status, got %q", cancelled.Appointment.Status)
	}

	items, err = store.Appointments(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin)
	if err != nil {
		t.Fatalf("list appointments after cancel: %v", err)
	}
	if items[0].Status != AppointmentCancelled {
		t.Fatalf("expected cancelled appointment to persist, got %#v", items[0])
	}
}

func TestDemoStoreClinicalDraftAndFinalizePersist(t *testing.T) {
	ctx := context.Background()
	store := NewDemoStore()

	created, err := store.CreateClinicalNote(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian, CreateClinicalNoteInput{
		LocationID:     "loc_001",
		PetID:          "pet_001",
		ReasonForVisit: "Stateful note test",
		Assessment:     "Stable",
	}, "clinical-note-test")
	if err != nil {
		t.Fatalf("create clinical note: %v", err)
	}

	items, err := store.ClinicalNotes(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian)
	if err != nil {
		t.Fatalf("list clinical notes: %v", err)
	}
	if len(items) == 0 || items[0].ID != created.ClinicalNote.ID || items[0].Status != "draft" {
		t.Fatalf("expected draft clinical note first, got %#v", items)
	}

	finalized, err := store.FinalizeClinicalNote(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian, created.ClinicalNote.ID, FinalizeClinicalNoteInput{
		ShareWithPetParent: true,
	}, "clinical-note-finalize-test")
	if err != nil {
		t.Fatalf("finalize clinical note: %v", err)
	}
	if finalized.ClinicalNote.Status != "finalized" || !finalized.ClinicalNote.SharedWithPetParent {
		t.Fatalf("expected finalized shared note, got %#v", finalized.ClinicalNote)
	}

	items, err = store.ClinicalNotes(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian)
	if err != nil {
		t.Fatalf("list clinical notes after finalize: %v", err)
	}
	if items[0].Status != "finalized" || !items[0].SharedWithPetParent {
		t.Fatalf("expected finalized note to persist, got %#v", items[0])
	}
}

func TestDemoStoreLabAndInvoiceLifecyclePersist(t *testing.T) {
	ctx := context.Background()
	store := NewDemoStore()

	lab, err := store.CreateLabOrder(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian, CreateLabOrderInput{
		LocationID: "loc_001",
		PetID:      "pet_001",
		TestType:   "Stateful chemistry panel",
	}, "lab-test")
	if err != nil {
		t.Fatalf("create lab order: %v", err)
	}
	updated, err := store.UploadLabResult(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian, lab.LabTest.ID, UploadLabResultInput{
		ReportObjectPath:   "reports/stateful-chemistry.pdf",
		ShareWithPetParent: true,
		MarkOrderCompleted: true,
	}, "lab-report-test")
	if err != nil {
		t.Fatalf("upload lab result: %v", err)
	}
	if updated.LabTest.Status != LabCompleted || !updated.LabTest.SharedWithPetParent {
		t.Fatalf("expected completed shared lab, got %#v", updated.LabTest)
	}
	labs, err := store.LabTests(ctx, "tenant_demo_clinic", "user_demo_doctor", RoleVeterinarian)
	if err != nil {
		t.Fatalf("list labs: %v", err)
	}
	if labs[0].Status != LabCompleted || labs[0].ReportURL == "" {
		t.Fatalf("expected completed lab to persist, got %#v", labs[0])
	}

	invoice, err := store.CreateInvoice(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin, CreateInvoiceInput{
		LocationID: "loc_001",
		PetID:      "pet_001",
		Status:     "issued",
		LineItems:  []InvoiceLineItemInput{{Description: "Stateful invoice", Quantity: 1, UnitAmountCents: 5000}},
	}, "invoice-test")
	if err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	voided, err := store.VoidInvoice(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin, invoice.Invoice.ID, VoidInvoiceInput{
		Reason: "Stateful void test",
	}, "invoice-void-test")
	if err != nil {
		t.Fatalf("void invoice: %v", err)
	}
	if voided.Invoice.Status != "void" {
		t.Fatalf("expected void invoice, got %#v", voided.Invoice)
	}
	billing, err := store.Billing(ctx, "tenant_demo_clinic", "user_demo_admin", RoleClinicAdmin)
	if err != nil {
		t.Fatalf("get billing: %v", err)
	}
	invoices, ok := billing["invoices"].([]Invoice)
	if !ok || len(invoices) == 0 || invoices[0].Status != "void" {
		t.Fatalf("expected void invoice to persist, got %#v", billing["invoices"])
	}
}

func TestZeroValueDemoStoreSeedsOnFirstUse(t *testing.T) {
	ctx := context.Background()
	var store DemoStore

	items, err := store.Appointments(ctx, "tenant_demo_clinic", "user_demo_parent", RolePetParent)
	if err != nil {
		t.Fatalf("list appointments: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected zero-value store to lazily seed pet parent appointment, got %#v", items)
	}
}
