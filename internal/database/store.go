package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pawit-vetcare/internal/domain"
)

type PostgresStore struct {
	pool *pgxpool.Pool
	demo domain.DemoStore
}

func NewPostgresStore(pool *pgxpool.Pool) PostgresStore {
	return PostgresStore{pool: pool, demo: domain.NewDemoStore()}
}

func (s PostgresStore) Ready(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s PostgresStore) ProductSpec(ctx context.Context) (domain.ProductSpec, error) {
	return s.demo.ProductSpec(ctx)
}

func (s PostgresStore) RolePolicies(ctx context.Context) ([]domain.RolePolicy, error) {
	return s.demo.RolePolicies(ctx)
}

func (s PostgresStore) Navigation(ctx context.Context, tenantID string) ([]domain.NavSection, error) {
	return s.demo.Navigation(ctx, tenantID)
}

func (s PostgresStore) Summary(ctx context.Context, tenantID string) ([]domain.Metric, error) {
	pets, err := s.count(ctx, "select count(*) from pets where tenant_id = $1 and archived_at is null", tenantID)
	if err != nil {
		return nil, err
	}
	appointments, err := s.count(ctx, "select count(*) from appointments where tenant_id = $1 and archived_at is null", tenantID)
	if err != nil {
		return nil, err
	}
	labs, err := s.count(ctx, "select count(*) from lab_orders where tenant_id = $1 and status <> 'completed' and cancelled_at is null", tenantID)
	if err != nil {
		return nil, err
	}

	var revenueCents int64
	if err := s.pool.QueryRow(ctx, "select coalesce(sum(total_cents), 0) from invoices where tenant_id = $1 and status = 'paid'", tenantID).Scan(&revenueCents); err != nil {
		return nil, fmt.Errorf("read revenue summary: %w", err)
	}

	return []domain.Metric{
		{Label: "Total Pets", Value: strconv.FormatInt(pets, 10), Delta: "Active pet patients", Tone: "blue"},
		{Label: "Appointments", Value: strconv.FormatInt(appointments, 10), Delta: "Tenant appointments", Tone: "green"},
		{Label: "Revenue", Value: formatCents(revenueCents), Delta: "From paid Stripe invoices", Tone: "green"},
		{Label: "Open Lab Tests", Value: strconv.FormatInt(labs, 10), Delta: "Pending diagnostics", Tone: "purple"},
	}, nil
}

func (s PostgresStore) Appointments(ctx context.Context, tenantID string) ([]domain.Appointment, error) {
	rows, err := s.pool.Query(ctx, `
		select
			a.id::text,
			p.name,
			coalesce(g.name, ''),
			coalesce(v.display_name, ''),
			coalesce(av.additional_vets, ''),
			coalesce(to_char(a.starts_at at time zone 'UTC', 'HH24:MI'), 'Unscheduled'),
			a.type::text,
			a.status::text,
			coalesce(g.email, ''),
			coalesce(a.telemedicine_url, ''),
			a.reason
		from appointments a
		join pets p on p.id = a.pet_id and p.tenant_id = a.tenant_id
		left join users v on v.id = a.primary_veterinarian_user_id
		left join lateral (
			select name, email
			from pet_guardians
			where tenant_id = a.tenant_id and pet_id = a.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		left join lateral (
			select string_agg(u.display_name, ', ' order by u.display_name) as additional_vets
			from appointment_veterinarians x
			join users u on u.id = x.veterinarian_user_id
			where x.appointment_id = a.id and x.is_primary = false
		) av on true
		where a.tenant_id = $1 and a.archived_at is null
		order by a.starts_at nulls last, a.created_at desc
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read appointments: %w", err)
	}
	defer rows.Close()

	items := []domain.Appointment{}
	for rows.Next() {
		var item domain.Appointment
		var appointmentType, status, additional string
		if err := rows.Scan(
			&item.ID,
			&item.PetName,
			&item.OwnerName,
			&item.PrimaryVeterinarian,
			&additional,
			&item.Time,
			&appointmentType,
			&status,
			&item.Contact,
			&item.MeetingURL,
			&item.Reason,
		); err != nil {
			return nil, fmt.Errorf("scan appointment: %w", err)
		}
		item.Type = domain.AppointmentType(appointmentType)
		item.Status = domain.AppointmentStatus(status)
		item.AdditionalVeterinarians = splitNames(additional)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate appointments: %w", err)
	}
	return items, nil
}

func (s PostgresStore) Calendar(ctx context.Context, tenantID string) (map[string]any, error) {
	items, err := s.Appointments(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{"scheduled": 0, "waiting": 0, "inProgress": 0, "done": 0}
	for _, item := range items {
		switch item.Status {
		case domain.AppointmentScheduled, domain.AppointmentConfirmed:
			counts["scheduled"]++
		case domain.AppointmentWaiting:
			counts["waiting"]++
		case domain.AppointmentInProgress:
			counts["inProgress"]++
		case domain.AppointmentCompleted:
			counts["done"]++
		}
	}
	return map[string]any{
		"date":         time.Now().UTC().Format(time.DateOnly),
		"statusCounts": counts,
		"items":        items,
	}, nil
}

func (s PostgresStore) Queue(ctx context.Context, tenantID string) ([]domain.QueueEntry, error) {
	rows, err := s.pool.Query(ctx, `
		select
			q.id::text,
			p.name,
			coalesce(g.name, ''),
			p.species::text,
			q.priority,
			q.status::text,
			greatest(0, floor(extract(epoch from (now() - q.checked_in_at)) / 60))::int
		from queue_entries q
		join pets p on p.id = q.pet_id and p.tenant_id = q.tenant_id
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = q.tenant_id and pet_id = q.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where q.tenant_id = $1 and q.cancelled_at is null and q.completed_at is null
		order by q.checked_in_at
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read queue: %w", err)
	}
	defer rows.Close()

	items := []domain.QueueEntry{}
	for rows.Next() {
		var item domain.QueueEntry
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Species, &item.Priority, &item.Status, &item.WaitMins); err != nil {
			return nil, fmt.Errorf("scan queue entry: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue: %w", err)
	}
	return items, nil
}

func (s PostgresStore) Patients(ctx context.Context, tenantID string) ([]domain.PatientRecord, error) {
	rows, err := s.pool.Query(ctx, `
		select
			p.id::text,
			p.name,
			coalesce(g.name, ''),
			p.species::text,
			coalesce(p.breed, ''),
			coalesce(p.estimated_age, ''),
			coalesce(p.sex, ''),
			coalesce(g.email, ''),
			coalesce(to_char(v.last_visit at time zone 'UTC', 'YYYY-MM-DD'), 'No visits'),
			(select count(*) from pet_guardians pg where pg.tenant_id = p.tenant_id and pg.pet_id = p.id and pg.archived_at is null)::int,
			(select count(*) from pet_documents pd where pd.tenant_id = p.tenant_id and pd.pet_id = p.id and pd.archived_at is null)::int
		from pets p
		left join lateral (
			select name, email
			from pet_guardians
			where tenant_id = p.tenant_id and pet_id = p.id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		left join lateral (
			select max(starts_at) as last_visit
			from appointments
			where tenant_id = p.tenant_id and pet_id = p.id and status = 'completed'
		) v on true
		where p.tenant_id = $1 and p.archived_at is null
		order by p.name
		limit 250
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read patients: %w", err)
	}
	defer rows.Close()

	items := []domain.PatientRecord{}
	for rows.Next() {
		var item domain.PatientRecord
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Species, &item.Breed, &item.Age, &item.Sex, &item.Phone, &item.LastVisit, &item.GuardianCount, &item.DocumentsCount); err != nil {
			return nil, fmt.Errorf("scan patient: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate patients: %w", err)
	}
	return items, nil
}

func (s PostgresStore) PrescriptionTemplates(ctx context.Context, tenantID string) ([]domain.PrescriptionTemplate, error) {
	return s.demo.PrescriptionTemplates(ctx, tenantID)
}

func (s PostgresStore) ClinicalNotes(ctx context.Context, tenantID string) ([]domain.ClinicalNote, error) {
	rows, err := s.pool.Query(ctx, `
		select
			n.id::text,
			p.name,
			coalesce(g.name, ''),
			coalesce(nullif(n.reason_for_visit, ''), nullif(n.assessment, ''), 'Clinical note'),
			n.status::text,
			n.updated_at,
			n.shared_with_pet_parent
		from clinical_notes n
		join pets p on p.id = n.pet_id and p.tenant_id = n.tenant_id
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = n.tenant_id and pet_id = n.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where n.tenant_id = $1 and n.archived_at is null
		order by n.updated_at desc
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read clinical notes: %w", err)
	}
	defer rows.Close()

	items := []domain.ClinicalNote{}
	for rows.Next() {
		var item domain.ClinicalNote
		var updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Subject, &item.Status, &updatedAt, &item.SharedWithPetParent); err != nil {
			return nil, fmt.Errorf("scan clinical note: %w", err)
		}
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clinical notes: %w", err)
	}
	return items, nil
}

func (s PostgresStore) LabTests(ctx context.Context, tenantID string) ([]domain.LabTest, error) {
	rows, err := s.pool.Query(ctx, `
		select
			o.id::text,
			p.name,
			coalesce(g.name, ''),
			o.test_type,
			coalesce(c.name, 'Internal lab'),
			coalesce(c.lab_type::text, 'internal'),
			o.status::text,
			o.shared_with_pet_parent
		from lab_orders o
		join pets p on p.id = o.pet_id and p.tenant_id = o.tenant_id
		left join lab_centers c on c.id = o.lab_center_id and c.tenant_id = o.tenant_id
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = o.tenant_id and pet_id = o.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where o.tenant_id = $1 and o.cancelled_at is null
		order by o.created_at desc
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read lab tests: %w", err)
	}
	defer rows.Close()

	items := []domain.LabTest{}
	for rows.Next() {
		var item domain.LabTest
		var status string
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.TestType, &item.LabCenter, &item.LabType, &status, &item.SharedWithPetParent); err != nil {
			return nil, fmt.Errorf("scan lab test: %w", err)
		}
		item.Status = domain.LabOrderStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate lab tests: %w", err)
	}
	return items, nil
}

func (s PostgresStore) Billing(ctx context.Context, tenantID string) (map[string]any, error) {
	today, err := s.sumCents(ctx, "select coalesce(sum(total_cents), 0) from invoices where tenant_id = $1 and status = 'paid' and paid_at::date = current_date", tenantID)
	if err != nil {
		return nil, err
	}
	pending, err := s.sumCents(ctx, "select coalesce(sum(total_cents), 0) from invoices where tenant_id = $1 and status in ('issued', 'pending')", tenantID)
	if err != nil {
		return nil, err
	}
	allTime, err := s.sumCents(ctx, "select coalesce(sum(total_cents), 0) from invoices where tenant_id = $1 and status = 'paid'", tenantID)
	if err != nil {
		return nil, err
	}
	overdue, err := s.count(ctx, "select count(*) from invoices where tenant_id = $1 and status in ('issued', 'pending') and due_at < now() - interval '30 days'", tenantID)
	if err != nil {
		return nil, err
	}
	invoices, err := s.invoices(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"metrics": []domain.Metric{
			{Label: "Total Revenue Today", Value: formatCents(today), Delta: "Paid invoices today", Tone: "green"},
			{Label: "Pending Payments", Value: formatCents(pending), Delta: "Issued and pending invoices", Tone: "orange"},
			{Label: "Total Revenue All Time", Value: formatCents(allTime), Delta: "Paid invoices", Tone: "green"},
			{Label: "Overdue Reminders", Value: strconv.FormatInt(overdue, 10), Delta: "Bills pending >30 days", Tone: "orange"},
		},
		"invoices": invoices,
	}, nil
}

func (s PostgresStore) Analytics(ctx context.Context, tenantID string) (domain.Analytics, error) {
	metrics, err := s.Summary(ctx, tenantID)
	if err != nil {
		return domain.Analytics{}, err
	}
	species, err := s.groupCount(ctx, "select species::text, count(*) from pets where tenant_id = $1 and archived_at is null group by species", tenantID)
	if err != nil {
		return domain.Analytics{}, err
	}
	appointmentStatus, err := s.groupCount(ctx, "select status::text, count(*) from appointments where tenant_id = $1 and archived_at is null group by status", tenantID)
	if err != nil {
		return domain.Analytics{}, err
	}
	return domain.Analytics{
		Metrics:             metrics,
		SpeciesDistribution: species,
		AppointmentStatus:   appointmentStatus,
		RevenueTrend:        map[string]string{},
		CommonDiagnoses:     []domain.Metric{},
	}, nil
}

func (s PostgresStore) Feedback(ctx context.Context, tenantID string) (map[string]any, error) {
	return s.demo.Feedback(ctx, tenantID)
}

func (s PostgresStore) Doctors(ctx context.Context, tenantID string) ([]domain.Person, error) {
	return s.people(ctx, tenantID, "Veterinarian")
}

func (s PostgresStore) Staff(ctx context.Context, tenantID string) ([]domain.Person, error) {
	rows, err := s.pool.Query(ctx, `
		select distinct on (u.id)
			u.id::text,
			u.display_name,
			r.code,
			u.email,
			m.status::text
		from tenant_memberships m
		join users u on u.id = m.user_id
		join membership_roles mr on mr.membership_id = m.id
		join roles r on r.id = mr.role_id
		where m.tenant_id = $1
			and m.archived_at is null
			and r.code not in ('Veterinarian', 'PetParent', 'SuperAdmin')
		order by u.id, r.code
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read staff: %w", err)
	}
	defer rows.Close()

	items := []domain.Person{}
	for rows.Next() {
		var item domain.Person
		if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.Email, &item.Status); err != nil {
			return nil, fmt.Errorf("scan staff member: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate staff: %w", err)
	}
	return items, nil
}

func (s PostgresStore) count(ctx context.Context, query string, tenantID string) (int64, error) {
	var value int64
	if err := s.pool.QueryRow(ctx, query, tenantID).Scan(&value); err != nil {
		return 0, fmt.Errorf("read count: %w", err)
	}
	return value, nil
}

func (s PostgresStore) sumCents(ctx context.Context, query string, tenantID string) (int64, error) {
	var value int64
	if err := s.pool.QueryRow(ctx, query, tenantID).Scan(&value); err != nil {
		return 0, fmt.Errorf("read amount: %w", err)
	}
	return value, nil
}

func (s PostgresStore) invoices(ctx context.Context, tenantID string) ([]domain.Invoice, error) {
	rows, err := s.pool.Query(ctx, `
		select
			i.id::text,
			coalesce(p.name, ''),
			coalesce(g.name, ''),
			i.total_cents,
			i.status::text,
			i.due_at
		from invoices i
		left join pets p on p.id = i.pet_id and p.tenant_id = i.tenant_id
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = i.tenant_id and pet_id = i.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where i.tenant_id = $1
		order by i.created_at desc
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read invoices: %w", err)
	}
	defer rows.Close()

	items := []domain.Invoice{}
	for rows.Next() {
		var item domain.Invoice
		var dueAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Amount, &item.Status, &dueAt); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		if dueAt.Valid {
			item.DueDate = dueAt.Time.UTC().Format(time.DateOnly)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoices: %w", err)
	}
	return items, nil
}

func (s PostgresStore) groupCount(ctx context.Context, query string, tenantID string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read grouped count: %w", err)
	}
	defer rows.Close()

	items := map[string]int{}
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return nil, fmt.Errorf("scan grouped count: %w", err)
		}
		items[key] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grouped count: %w", err)
	}
	return items, nil
}

func (s PostgresStore) people(ctx context.Context, tenantID string, role string) ([]domain.Person, error) {
	rows, err := s.pool.Query(ctx, `
		select
			u.id::text,
			u.display_name,
			r.code,
			'Small Animal Medicine',
			u.email,
			m.status::text
		from tenant_memberships m
		join users u on u.id = m.user_id
		join membership_roles mr on mr.membership_id = m.id
		join roles r on r.id = mr.role_id
		where m.tenant_id = $1 and m.archived_at is null and r.code = $2
		order by u.display_name
		limit 100
	`, tenantID, role)
	if err != nil {
		return nil, fmt.Errorf("read people: %w", err)
	}
	defer rows.Close()

	items := []domain.Person{}
	for rows.Next() {
		var item domain.Person
		if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.Specialty, &item.Email, &item.Status); err != nil {
			return nil, fmt.Errorf("scan person: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate people: %w", err)
	}
	return items, nil
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

func splitNames(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}
