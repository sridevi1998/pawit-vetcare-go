package database

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pawit-vetcare/internal/domain"
)

type PostgresStore struct {
	pool           *pgxpool.Pool
	demo           domain.DemoStore
	documentBucket string
	signingEmail   string
	privateKeyPEM  string
}

const (
	httpStatusOK      = 200
	httpStatusCreated = 201
	maxDocumentBytes  = 25 * 1024 * 1024
)

type DocumentStorageConfig struct {
	Bucket        string
	SigningEmail  string
	PrivateKeyPEM string
}

func NewPostgresStore(pool *pgxpool.Pool, options ...DocumentStorageConfig) PostgresStore {
	bucket := "pawit-vetcare-documents-dev"
	if len(options) > 0 && strings.TrimSpace(options[0].Bucket) != "" {
		bucket = strings.TrimSpace(options[0].Bucket)
	}
	store := PostgresStore{pool: pool, demo: domain.NewDemoStore(), documentBucket: bucket}
	if len(options) > 0 {
		store.signingEmail = strings.TrimSpace(options[0].SigningEmail)
		store.privateKeyPEM = strings.TrimSpace(options[0].PrivateKeyPEM)
	}
	return store
}

func (s PostgresStore) Ready(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s PostgresStore) Authenticate(ctx context.Context, input domain.LoginInput) (domain.AuthIdentity, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	password := strings.TrimSpace(input.Password)
	tenantRef := strings.TrimSpace(input.TenantID)
	if tenantRef == "" {
		tenantRef = strings.TrimSpace(input.HospitalID)
	}
	role := input.Role
	if email == "" || password == "" {
		return domain.AuthIdentity{}, domain.ErrInvalidCredentials
	}

	query := `
		select
			u.id::text,
			u.email,
			u.display_name,
			u.password_hash,
			m.tenant_id::text,
			r.code
		from users u
		join tenant_memberships m on m.user_id = u.id
		join tenants t on t.id = m.tenant_id
		join membership_roles mr on mr.membership_id = m.id
		join roles r on r.id = mr.role_id
		where u.email_normalized = $1
			and u.status = 'active'
			and u.archived_at is null
			and m.status = 'active'
			and m.archived_at is null
			and (
				$2::text = ''
				or m.tenant_id::text = $2
				or lower(t.name) = lower($2)
				or lower(replace(t.name, ' ', '-')) = lower($2)
			)
			and ($3::text = '' or r.code = $3)
		order by
			case r.code
				when 'ClinicAdmin' then 1
				when 'Veterinarian' then 2
				when 'Receptionist' then 3
				else 10
			end,
			r.code
		limit 1
	`

	var identity domain.AuthIdentity
	var passwordHash string
	var roleCode string
	err := s.pool.QueryRow(ctx, query, email, tenantRef, string(role)).Scan(
		&identity.UserID,
		&identity.Email,
		&identity.DisplayName,
		&passwordHash,
		&identity.TenantID,
		&roleCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AuthIdentity{}, domain.ErrInvalidCredentials
	}
	if err != nil {
		return domain.AuthIdentity{}, fmt.Errorf("read login identity: %w", err)
	}
	if !passwordMatches(passwordHash, password) {
		return domain.AuthIdentity{}, domain.ErrInvalidCredentials
	}

	identity.Role = domain.Role(roleCode)
	if _, err := s.pool.Exec(ctx, "update users set last_login_at = now(), updated_at = now() where id = $1", identity.UserID); err != nil {
		return domain.AuthIdentity{}, fmt.Errorf("record login: %w", err)
	}
	return identity, nil
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

func (s PostgresStore) Locations(ctx context.Context, tenantID string) ([]domain.ClinicLocation, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id::text,
			name,
			timezone,
			coalesce(phone, ''),
			coalesce(email, ''),
			status::text
		from clinic_locations
		where tenant_id = $1
			and archived_at is null
			and status = 'active'
		order by name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read clinic locations: %w", err)
	}
	defer rows.Close()

	items := []domain.ClinicLocation{}
	for rows.Next() {
		var item domain.ClinicLocation
		if err := rows.Scan(&item.ID, &item.Name, &item.Timezone, &item.Phone, &item.Email, &item.Status); err != nil {
			return nil, fmt.Errorf("scan clinic location: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clinic locations: %w", err)
	}
	return items, nil
}

func (s PostgresStore) Tenants(ctx context.Context) ([]domain.Tenant, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id::text,
			name,
			coalesce(legal_name, ''),
			status::text,
			coalesce(stripe_customer_id, ''),
			default_cancellation_cutoff_hours,
			created_at,
			updated_at
		from tenants
		where archived_at is null
		order by created_at desc
		limit 100
	`)
	if err != nil {
		return nil, fmt.Errorf("read tenants: %w", err)
	}
	defer rows.Close()

	items := []domain.Tenant{}
	for rows.Next() {
		var item domain.Tenant
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.Name, &item.LegalName, &item.Status, &item.StripeCustomerID, &item.DefaultCancellationCutoffHours, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		locations, err := locationsForTenant(ctx, s.pool, item.ID, false)
		if err != nil {
			return nil, err
		}
		item.Locations = locations
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenants: %w", err)
	}
	return items, nil
}

func (s PostgresStore) Tenant(ctx context.Context, tenantID string) (domain.Tenant, error) {
	return tenantByID(ctx, s.pool, tenantID)
}

func (s PostgresStore) CreateTenant(ctx context.Context, actorTenantID string, actorUserID string, actorRole domain.Role, input domain.CreateTenantInput, idempotencyKey string) (domain.TenantMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionTenantManage) {
		return domain.TenantMutationResult{}, domain.ErrForbidden
	}
	if !isUUID(actorTenantID) || !isUUID(actorUserID) {
		return domain.TenantMutationResult{}, fmt.Errorf("%w: actor tenant and user ids must be UUIDs", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("begin tenant create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentTenantResult(ctx, tx, actorTenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	cutoff := input.DefaultCancellationCutoffHours
	if cutoff == 0 {
		cutoff = 24
	}

	var tenantID string
	err = tx.QueryRow(ctx, `
		insert into tenants (name, legal_name, default_cancellation_cutoff_hours, status)
		values ($1, $2, $3, 'active')
		returning id::text
	`, strings.TrimSpace(input.Name), nullString(input.LegalName), cutoff).Scan(&tenantID)
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("create tenant: %w", err)
	}

	var locationID string
	err = tx.QueryRow(ctx, `
		insert into clinic_locations (
			tenant_id,
			name,
			timezone,
			phone,
			email,
			cancellation_cutoff_hours,
			status
		)
		values ($1, $2, $3, $4, $5, $6, 'active')
		returning id::text
	`,
		tenantID,
		strings.TrimSpace(input.FirstLocation.Name),
		strings.TrimSpace(input.FirstLocation.Timezone),
		nullString(input.FirstLocation.Phone),
		nullString(input.FirstLocation.Email),
		nullableInt(input.FirstLocation.CancellationCutoffHours),
	).Scan(&locationID)
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("create first clinic location: %w", err)
	}

	adminEmail := strings.ToLower(strings.TrimSpace(input.FirstAdmin.Email))
	adminStatus := "invited"
	passwordHash := "invited:pending"
	if strings.TrimSpace(input.FirstAdmin.TemporaryPassword) != "" {
		adminStatus = "active"
		passwordHash, err = hashPassword(strings.TrimSpace(input.FirstAdmin.TemporaryPassword))
		if err != nil {
			return domain.TenantMutationResult{}, err
		}
	}

	var adminUserID string
	err = tx.QueryRow(ctx, `
		insert into users (email, password_hash, display_name, status)
		values ($1, $2, $3, $4)
		on conflict (email_normalized) do update
		set display_name = excluded.display_name,
			password_hash = case when excluded.password_hash = 'invited:pending' then users.password_hash else excluded.password_hash end,
			status = case when users.status = 'archived' then excluded.status else users.status end,
			archived_at = null,
			updated_at = now()
		returning id::text
	`, adminEmail, passwordHash, strings.TrimSpace(input.FirstAdmin.Name), adminStatus).Scan(&adminUserID)
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("create first clinic admin user: %w", err)
	}

	var membershipID string
	err = tx.QueryRow(ctx, `
		insert into tenant_memberships (tenant_id, user_id, default_location_id, status)
		values ($1, $2, $3, $4)
		on conflict (tenant_id, user_id) do update
		set default_location_id = excluded.default_location_id,
			status = excluded.status,
			archived_at = null,
			updated_at = now()
		returning id::text
	`, tenantID, adminUserID, locationID, adminStatus).Scan(&membershipID)
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("create first clinic admin membership: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into membership_roles (membership_id, role_id)
		select $1, id from roles where code = 'ClinicAdmin'
		on conflict do nothing
	`, membershipID); err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("assign first clinic admin role: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "tenant.create", "tenant", tenantID, strings.TrimSpace(input.Name)); err != nil {
		return domain.TenantMutationResult{}, err
	}
	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "clinic_location.create", "clinic_location", locationID, strings.TrimSpace(input.FirstLocation.Name)); err != nil {
		return domain.TenantMutationResult{}, err
	}
	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "staff.create", "user", adminUserID, "ClinicAdmin"); err != nil {
		return domain.TenantMutationResult{}, err
	}

	tenant, err := tenantByID(ctx, tx, tenantID)
	if err != nil {
		return domain.TenantMutationResult{}, err
	}
	result := domain.TenantMutationResult{Tenant: tenant}
	if err := rememberTenantIdempotentResult(ctx, tx, actorTenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.TenantMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("commit tenant create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) UpdateTenant(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.UpdateTenantInput, idempotencyKey string) (domain.TenantMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionTenantManage) {
		return domain.TenantMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("begin tenant update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		TenantID string
		Input    domain.UpdateTenantInput
	}{TenantID: tenantID, Input: input})
	if result, ok, err := idempotentTenantResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	status := input.Status
	archivedExpr := "archived_at"
	if status == "archived" {
		archivedExpr = "coalesce(archived_at, now())"
	} else if status == "active" || status == "suspended" {
		archivedExpr = "null"
	}

	command, err := tx.Exec(ctx, fmt.Sprintf(`
		update tenants
		set name = coalesce($2, name),
			legal_name = coalesce($3, legal_name),
			status = coalesce($4::tenant_status, status),
			default_cancellation_cutoff_hours = coalesce($5, default_cancellation_cutoff_hours),
			archived_at = %s,
			updated_at = now()
		where id = $1
	`, archivedExpr), tenantID, nullString(input.Name), nullString(input.LegalName), nullString(status), nullableInt(input.DefaultCancellationCutoffHours))
	if err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("update tenant: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.TenantMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "tenant.update", "tenant", tenantID, status); err != nil {
		return domain.TenantMutationResult{}, err
	}
	tenant, err := tenantByID(ctx, tx, tenantID)
	if err != nil {
		return domain.TenantMutationResult{}, err
	}
	result := domain.TenantMutationResult{Tenant: tenant}
	if err := rememberTenantIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.TenantMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.TenantMutationResult{}, fmt.Errorf("commit tenant update: %w", err)
	}
	return result, nil
}

func (s PostgresStore) CreateTenantLocation(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateClinicLocationInput, idempotencyKey string) (domain.ClinicLocationMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionTenantManage) {
		return domain.ClinicLocationMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ClinicLocationMutationResult{}, fmt.Errorf("begin tenant location create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentClinicLocationResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var locationID string
	err = tx.QueryRow(ctx, `
		insert into clinic_locations (
			tenant_id,
			name,
			timezone,
			phone,
			email,
			cancellation_cutoff_hours,
			status
		)
		values ($1, $2, $3, $4, $5, $6, 'active')
		returning id::text
	`, tenantID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Timezone), nullString(input.Phone), nullString(input.Email), nullableInt(input.CancellationCutoffHours)).Scan(&locationID)
	if err != nil {
		return domain.ClinicLocationMutationResult{}, fmt.Errorf("create clinic location: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "clinic_location.create", "clinic_location", locationID, input.Name); err != nil {
		return domain.ClinicLocationMutationResult{}, err
	}
	location, err := clinicLocationByID(ctx, tx, tenantID, locationID)
	if err != nil {
		return domain.ClinicLocationMutationResult{}, err
	}
	result := domain.ClinicLocationMutationResult{Location: location}
	if err := rememberClinicLocationIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.ClinicLocationMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ClinicLocationMutationResult{}, fmt.Errorf("commit tenant location create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) UpdateTenantLocation(ctx context.Context, tenantID string, locationID string, actorUserID string, actorRole domain.Role, input domain.UpdateClinicLocationInput, idempotencyKey string) (domain.ClinicLocationMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionTenantManage) {
		return domain.ClinicLocationMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ClinicLocationMutationResult{}, fmt.Errorf("begin tenant location update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		LocationID string
		Input      domain.UpdateClinicLocationInput
	}{LocationID: locationID, Input: input})
	if result, ok, err := idempotentClinicLocationResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	status := input.Status
	archivedExpr := "archived_at"
	if status == "archived" {
		archivedExpr = "coalesce(archived_at, now())"
	} else if status == "active" || status == "suspended" {
		archivedExpr = "null"
	}
	command, err := tx.Exec(ctx, fmt.Sprintf(`
		update clinic_locations
		set name = coalesce($3, name),
			timezone = coalesce($4, timezone),
			phone = coalesce($5, phone),
			email = coalesce($6, email),
			cancellation_cutoff_hours = coalesce($7, cancellation_cutoff_hours),
			status = coalesce($8::tenant_status, status),
			archived_at = %s,
			updated_at = now()
		where tenant_id = $1 and id = $2
	`, archivedExpr), tenantID, locationID, nullString(input.Name), nullString(input.Timezone), nullString(input.Phone), nullString(input.Email), nullableInt(input.CancellationCutoffHours), nullString(status))
	if err != nil {
		return domain.ClinicLocationMutationResult{}, fmt.Errorf("update clinic location: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.ClinicLocationMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "clinic_location.update", "clinic_location", locationID, status); err != nil {
		return domain.ClinicLocationMutationResult{}, err
	}
	location, err := clinicLocationByID(ctx, tx, tenantID, locationID)
	if err != nil {
		return domain.ClinicLocationMutationResult{}, err
	}
	result := domain.ClinicLocationMutationResult{Location: location}
	if err := rememberClinicLocationIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.ClinicLocationMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ClinicLocationMutationResult{}, fmt.Errorf("commit tenant location update: %w", err)
	}
	return result, nil
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

func (s PostgresStore) Appointments(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.Appointment, error) {
	ownOnly, err := ownAppointmentReadOnly(actorRole)
	if err != nil {
		return nil, err
	}

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
		where a.tenant_id = $1
			and a.archived_at is null
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = a.tenant_id
						and pg.pet_id = a.pet_id
						and pg.user_id = $3
						and pg.archived_at is null
				)
			)
		order by a.starts_at nulls last, a.created_at desc
		limit 100
	`, tenantID, ownOnly, uuidOrNil(actorUserID))
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

func (s PostgresStore) CreateAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateAppointmentInput, idempotencyKey string) (domain.AppointmentMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionAppointmentManage, domain.PermissionAppointmentRequestOwn) {
		return domain.AppointmentMutationResult{}, domain.ErrForbidden
	}

	status := domain.AppointmentRequested
	if actorRole != domain.RolePetParent && !input.RequestedByPetParent && input.StartsAt != nil {
		status = domain.AppointmentScheduled
	}
	if input.Type == domain.AppointmentTelemedicine && status != domain.AppointmentRequested && strings.TrimSpace(input.MeetingURL) == "" {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: meetingUrl is required before scheduling telemedicine appointments", domain.ErrValidation)
	}

	startsAt, err := parseOptionalTime(input.StartsAt)
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: startsAt must be RFC3339", domain.ErrValidation)
	}
	endsAt, err := parseOptionalTime(input.EndsAt)
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: endsAt must be RFC3339", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("begin appointment create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var appointmentID string
	err = tx.QueryRow(ctx, `
		insert into appointments (
			tenant_id,
			location_id,
			pet_id,
			requested_by_user_id,
			primary_veterinarian_user_id,
			type,
			status,
			starts_at,
			ends_at,
			reason,
			telemedicine_url
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		returning id::text
	`,
		tenantID,
		input.LocationID,
		input.PetID,
		uuidOrNil(actorUserID),
		uuidOrNil(input.PrimaryVeterinarianID),
		string(input.Type),
		string(status),
		startsAt,
		endsAt,
		strings.TrimSpace(input.Reason),
		nullString(input.MeetingURL),
	).Scan(&appointmentID)
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("create appointment: %w", err)
	}

	if isUUID(input.PrimaryVeterinarianID) {
		if _, err := tx.Exec(ctx, `
			insert into appointment_veterinarians (appointment_id, veterinarian_user_id, is_primary)
			values ($1, $2, true)
			on conflict (appointment_id, veterinarian_user_id) do update set is_primary = excluded.is_primary
		`, appointmentID, input.PrimaryVeterinarianID); err != nil {
			return domain.AppointmentMutationResult{}, fmt.Errorf("assign primary veterinarian: %w", err)
		}
	}

	for _, vetID := range input.AdditionalVeterinarianIDs {
		if !isUUID(vetID) || vetID == input.PrimaryVeterinarianID {
			continue
		}
		if _, err := tx.Exec(ctx, `
			insert into appointment_veterinarians (appointment_id, veterinarian_user_id, is_primary)
			values ($1, $2, false)
			on conflict (appointment_id, veterinarian_user_id) do nothing
		`, appointmentID, vetID); err != nil {
			return domain.AppointmentMutationResult{}, fmt.Errorf("assign additional veterinarian: %w", err)
		}
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "appointment.create", "appointment", appointmentID, input.Reason); err != nil {
		return domain.AppointmentMutationResult{}, err
	}

	appointment, err := appointmentByID(ctx, tx, tenantID, appointmentID)
	if err != nil {
		return domain.AppointmentMutationResult{}, err
	}
	result := domain.AppointmentMutationResult{Appointment: appointment}
	if err := rememberIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.AppointmentMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("commit appointment create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) CancelAppointment(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, appointmentID string, input domain.CancelAppointmentInput, idempotencyKey string) (domain.AppointmentMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionAppointmentManage, domain.PermissionAppointmentRequestOwn) {
		return domain.AppointmentMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("begin appointment cancel: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		AppointmentID string
		Input         domain.CancelAppointmentInput
	}{AppointmentID: appointmentID, Input: input})
	if result, ok, err := idempotentResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var startsAt sql.NullTime
	var status string
	var cutoffHours int
	err = tx.QueryRow(ctx, `
		select
			a.starts_at,
			a.status::text,
			coalesce(l.cancellation_cutoff_hours, t.default_cancellation_cutoff_hours)
		from appointments a
		join tenants t on t.id = a.tenant_id
		join clinic_locations l on l.id = a.location_id and l.tenant_id = a.tenant_id
		where a.tenant_id = $1 and a.id = $2 and a.archived_at is null
		for update
	`, tenantID, appointmentID).Scan(&startsAt, &status, &cutoffHours)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AppointmentMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("read appointment cancellation policy: %w", err)
	}
	if status == string(domain.AppointmentCancelled) {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: appointment is already cancelled", domain.ErrConflict)
	}
	if status == string(domain.AppointmentCompleted) {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: completed appointments cannot be cancelled", domain.ErrConflict)
	}

	insideCutoff := startsAt.Valid && time.Until(startsAt.Time) < time.Duration(cutoffHours)*time.Hour
	if insideCutoff && !roleHasAny(actorRole, domain.PermissionAppointmentManage) {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: staff override is not allowed for this role", domain.ErrForbidden)
	}
	if insideCutoff && !input.StaffOverride {
		return domain.AppointmentMutationResult{}, fmt.Errorf("%w: appointment is inside the clinic cancellation cutoff", domain.ErrForbidden)
	}

	command, err := tx.Exec(ctx, `
		update appointments
		set status = 'cancelled',
			cancellation_reason = $3,
			cancelled_at = now(),
			updated_at = now()
		where tenant_id = $1 and id = $2 and archived_at is null
	`, tenantID, appointmentID, strings.TrimSpace(input.Reason))
	if err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("cancel appointment: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.AppointmentMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "appointment.cancel", "appointment", appointmentID, input.Reason); err != nil {
		return domain.AppointmentMutationResult{}, err
	}

	appointment, err := appointmentByID(ctx, tx, tenantID, appointmentID)
	if err != nil {
		return domain.AppointmentMutationResult{}, err
	}
	result := domain.AppointmentMutationResult{Appointment: appointment}
	if err := rememberIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.AppointmentMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.AppointmentMutationResult{}, fmt.Errorf("commit appointment cancel: %w", err)
	}
	return result, nil
}

func (s PostgresStore) Calendar(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) (map[string]any, error) {
	items, err := s.Appointments(ctx, tenantID, actorUserID, actorRole)
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
		var status string
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Species, &item.Priority, &status, &item.WaitMins); err != nil {
			return nil, fmt.Errorf("scan queue entry: %w", err)
		}
		item.Status = domain.QueueStatus(status)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue: %w", err)
	}
	return items, nil
}

func (s PostgresStore) RegisterWalkIn(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.RegisterWalkInInput, idempotencyKey string) (domain.QueueMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionQueueManage) {
		return domain.QueueMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("begin walk-in registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentQueueResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var activeCount int
	err = tx.QueryRow(ctx, `
		select count(*)
		from queue_entries
		where tenant_id = $1
			and location_id = $2
			and pet_id = $3
			and status in ('waiting', 'called', 'in_progress')
			and cancelled_at is null
			and completed_at is null
	`, tenantID, input.LocationID, input.PetID).Scan(&activeCount)
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("check active queue entry: %w", err)
	}
	if activeCount > 0 {
		return domain.QueueMutationResult{}, fmt.Errorf("%w: pet already has an active queue entry at this location", domain.ErrConflict)
	}

	var appointmentID string
	err = tx.QueryRow(ctx, `
		insert into appointments (
			tenant_id,
			location_id,
			pet_id,
			requested_by_user_id,
			type,
			status,
			reason
		)
		values ($1, $2, $3, $4, 'walk_in', 'waiting', $5)
		returning id::text
	`, tenantID, input.LocationID, input.PetID, uuidOrNil(actorUserID), strings.TrimSpace(input.Reason)).Scan(&appointmentID)
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("create walk-in appointment: %w", err)
	}

	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = "normal"
	}

	var queueID string
	err = tx.QueryRow(ctx, `
		insert into queue_entries (
			tenant_id,
			location_id,
			appointment_id,
			pet_id,
			status,
			priority,
			reason
		)
		values ($1, $2, $3, $4, 'waiting', $5, $6)
		returning id::text
	`, tenantID, input.LocationID, appointmentID, input.PetID, priority, nullString(input.Reason)).Scan(&queueID)
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("create queue entry: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "queue.walk_in.register", "queue_entry", queueID, input.Reason); err != nil {
		return domain.QueueMutationResult{}, err
	}

	entry, err := queueEntryByID(ctx, tx, tenantID, queueID)
	if err != nil {
		return domain.QueueMutationResult{}, err
	}
	result := domain.QueueMutationResult{QueueEntry: entry}
	if err := rememberQueueIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.QueueMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("commit walk-in registration: %w", err)
	}
	return result, nil
}

func (s PostgresStore) UpdateQueueStatus(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, queueID string, status domain.QueueStatus, input domain.UpdateQueueInput, idempotencyKey string) (domain.QueueMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionQueueManage) {
		return domain.QueueMutationResult{}, domain.ErrForbidden
	}
	if !validQueueStatus(status) || status == domain.QueueWaiting {
		return domain.QueueMutationResult{}, fmt.Errorf("%w: unsupported queue status transition", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("begin queue update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		QueueID string
		Status  domain.QueueStatus
		Input   domain.UpdateQueueInput
	}{QueueID: queueID, Status: status, Input: input})
	if result, ok, err := idempotentQueueResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var currentStatus string
	var appointmentID sql.NullString
	err = tx.QueryRow(ctx, `
		select status::text, appointment_id::text
		from queue_entries
		where tenant_id = $1 and id = $2
		for update
	`, tenantID, queueID).Scan(&currentStatus, &appointmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueueMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("read queue entry: %w", err)
	}

	if !queueTransitionAllowed(domain.QueueStatus(currentStatus), status) {
		return domain.QueueMutationResult{}, fmt.Errorf("%w: queue entry cannot move from %s to %s", domain.ErrConflict, currentStatus, status)
	}

	updateSQL, appointmentStatus := queueStatusUpdateSQL(status)
	command, err := tx.Exec(ctx, updateSQL, tenantID, queueID)
	if err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("update queue entry: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.QueueMutationResult{}, domain.ErrNotFound
	}

	if appointmentID.Valid && appointmentStatus != "" {
		if _, err := tx.Exec(ctx, `
			update appointments
			set status = $3,
				updated_at = now()
			where tenant_id = $1 and id = $2 and archived_at is null
		`, tenantID, appointmentID.String, appointmentStatus); err != nil {
			return domain.QueueMutationResult{}, fmt.Errorf("update queue appointment status: %w", err)
		}
	}
	if status == domain.QueueCancelled && appointmentID.Valid {
		if _, err := tx.Exec(ctx, `
			update appointments
			set status = 'cancelled',
				cancellation_reason = $3,
				cancelled_at = now(),
				updated_at = now()
			where tenant_id = $1 and id = $2 and archived_at is null
		`, tenantID, appointmentID.String, nullString(input.Reason)); err != nil {
			return domain.QueueMutationResult{}, fmt.Errorf("cancel queue appointment: %w", err)
		}
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "queue."+string(status), "queue_entry", queueID, input.Reason); err != nil {
		return domain.QueueMutationResult{}, err
	}

	entry, err := queueEntryByID(ctx, tx, tenantID, queueID)
	if err != nil {
		return domain.QueueMutationResult{}, err
	}
	result := domain.QueueMutationResult{QueueEntry: entry}
	if err := rememberQueueIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.QueueMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.QueueMutationResult{}, fmt.Errorf("commit queue update: %w", err)
	}
	return result, nil
}

func (s PostgresStore) Patients(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.PatientRecord, error) {
	ownOnly, err := ownPetRecordReadOnly(actorRole)
	if err != nil {
		return nil, err
	}

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
		where p.tenant_id = $1
			and p.archived_at is null
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = p.tenant_id
						and pg.pet_id = p.id
						and pg.user_id = $3
						and pg.can_view_records = true
						and pg.archived_at is null
				)
			)
		order by p.name
		limit 250
	`, tenantID, ownOnly, uuidOrNil(actorUserID))
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

func (s PostgresStore) CreatePet(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreatePetInput, idempotencyKey string) (domain.PetMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPetRecordManage, domain.PermissionPetRecordManageOwn) {
		return domain.PetMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("begin pet create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentPetResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var petID string
	err = tx.QueryRow(ctx, `
		insert into pets (
			tenant_id,
			primary_location_id,
			name,
			species,
			breed,
			sex,
			estimated_age
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning id::text
	`,
		tenantID,
		input.LocationID,
		strings.TrimSpace(input.Name),
		string(input.Species),
		nullString(input.Breed),
		nullString(input.Sex),
		nullString(input.EstimatedAge),
	).Scan(&petID)
	if err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("create pet: %w", err)
	}

	relationship := strings.TrimSpace(input.Relationship)
	if relationship == "" {
		relationship = "guardian"
	}
	isPrimary := input.PrimaryGuardian
	if !isPrimary {
		isPrimary = true
	}
	var guardianUserID any
	if actorRole == domain.RolePetParent {
		guardianUserID = uuidOrNil(actorUserID)
	}

	if _, err := tx.Exec(ctx, `
		insert into pet_guardians (
			tenant_id,
			pet_id,
			user_id,
			name,
			email,
			relationship,
			is_primary
		)
		values ($1, $2, $3, $4, $5, $6, $7)
	`,
		tenantID,
		petID,
		guardianUserID,
		strings.TrimSpace(input.GuardianName),
		nullString(input.GuardianEmail),
		relationship,
		isPrimary,
	); err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("create pet guardian: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "pet.create", "pet", petID, input.Name); err != nil {
		return domain.PetMutationResult{}, err
	}

	pet, err := patientByID(ctx, tx, tenantID, petID)
	if err != nil {
		return domain.PetMutationResult{}, err
	}
	result := domain.PetMutationResult{Pet: pet}
	if err := rememberPetIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.PetMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("commit pet create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) ArchivePet(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, input domain.ArchivePetInput, idempotencyKey string) (domain.PetMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPetRecordManage) {
		return domain.PetMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("begin pet archive: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		PetID string
		Input domain.ArchivePetInput
	}{PetID: petID, Input: input})
	if result, ok, err := idempotentPetResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	pet, err := patientByID(ctx, tx, tenantID, petID)
	if err != nil {
		return domain.PetMutationResult{}, err
	}

	command, err := tx.Exec(ctx, `
		update pets
		set status = 'archived',
			archived_at = now(),
			updated_at = now()
		where tenant_id = $1 and id = $2 and archived_at is null
	`, tenantID, petID)
	if err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("archive pet: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.PetMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "pet.archive", "pet", petID, input.Reason); err != nil {
		return domain.PetMutationResult{}, err
	}

	result := domain.PetMutationResult{Pet: pet}
	if err := rememberPetIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.PetMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PetMutationResult{}, fmt.Errorf("commit pet archive: %w", err)
	}
	return result, nil
}

func (s PostgresStore) PetDocuments(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string) ([]domain.PetDocument, error) {
	ownOnly, err := ownPetRecordReadOnly(actorRole)
	if err != nil && !roleHasAny(actorRole, domain.PermissionPetDocumentUpload) {
		return nil, err
	}
	if _, err := patientByID(ctx, s.pool, tenantID, petID); err != nil {
		return nil, err
	}
	if ownOnly || (actorRole == domain.RolePetParent && !roleHasAny(actorRole, domain.PermissionPetRecordManage)) {
		if err := requirePetGuardianAccess(ctx, s.pool, tenantID, petID, actorUserID, true); err != nil {
			return nil, err
		}
	}

	rows, err := s.pool.Query(ctx, `
		select
			id::text,
			pet_id::text,
			title,
			document_type,
			object_path,
			content_type,
			size_bytes,
			status::text,
			created_at
		from pet_documents
		where tenant_id = $1 and pet_id = $2 and archived_at is null
		order by created_at desc
		limit 100
	`, tenantID, petID)
	if err != nil {
		return nil, fmt.Errorf("read pet documents: %w", err)
	}
	defer rows.Close()

	items := []domain.PetDocument{}
	for rows.Next() {
		var item domain.PetDocument
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.PetID, &item.Title, &item.DocumentType, &item.ObjectPath, &item.ContentType, &item.SizeBytes, &item.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("scan pet document: %w", err)
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pet documents: %w", err)
	}
	return items, nil
}

func (s PostgresStore) PreparePetDocumentUpload(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, input domain.PreparePetDocumentUploadInput, idempotencyKey string) (domain.PetDocumentUploadURLResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPetDocumentUpload) {
		return domain.PetDocumentUploadURLResult{}, domain.ErrForbidden
	}
	if input.SizeBytes <= 0 || input.SizeBytes > maxDocumentBytes {
		return domain.PetDocumentUploadURLResult{}, fmt.Errorf("%w: sizeBytes must be between 1 and %d", domain.ErrValidation, maxDocumentBytes)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PetDocumentUploadURLResult{}, fmt.Errorf("begin pet document upload url: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		PetID string
		Input domain.PreparePetDocumentUploadInput
	}{PetID: petID, Input: input})
	if result, ok, err := idempotentPetDocumentUploadURLResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	if _, err := patientByID(ctx, tx, tenantID, petID); err != nil {
		return domain.PetDocumentUploadURLResult{}, err
	}
	if actorRole == domain.RolePetParent {
		if err := requirePetGuardianAccess(ctx, tx, tenantID, petID, actorUserID, true); err != nil {
			return domain.PetDocumentUploadURLResult{}, err
		}
	}

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	objectPath := documentObjectPath(tenantID, petID, input.Title, input.ContentType)
	uploadURL, err := s.signedStorageURL(objectPath, "PUT", expiresAt)
	if err != nil {
		return domain.PetDocumentUploadURLResult{}, err
	}
	result := domain.PetDocumentUploadURLResult{
		ObjectPath: objectPath,
		UploadURL:  uploadURL,
		Method:     "PUT",
		Headers: map[string]string{
			"Content-Type": strings.TrimSpace(input.ContentType),
		},
		ExpiresAt:    expiresAt.Format(time.RFC3339),
		MaxSizeBytes: maxDocumentBytes,
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "pet_document.upload_url", "pet", petID, input.Title); err != nil {
		return domain.PetDocumentUploadURLResult{}, err
	}
	if err := rememberPetDocumentUploadURLResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.PetDocumentUploadURLResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PetDocumentUploadURLResult{}, fmt.Errorf("commit pet document upload url: %w", err)
	}
	return result, nil
}

func (s PostgresStore) UploadPetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, input domain.UploadPetDocumentInput, idempotencyKey string) (domain.PetDocumentMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPetDocumentUpload) {
		return domain.PetDocumentMutationResult{}, domain.ErrForbidden
	}
	if actorRole == domain.RolePetParent {
		if err := requirePetGuardianAccess(ctx, s.pool, tenantID, petID, actorUserID, true); err != nil {
			return domain.PetDocumentMutationResult{}, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PetDocumentMutationResult{}, fmt.Errorf("begin pet document upload: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		PetID string
		Input domain.UploadPetDocumentInput
	}{PetID: petID, Input: input})
	if result, ok, err := idempotentPetDocumentResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	if _, err := patientByID(ctx, tx, tenantID, petID); err != nil {
		return domain.PetDocumentMutationResult{}, err
	}

	var documentID string
	err = tx.QueryRow(ctx, `
		insert into pet_documents (
			tenant_id,
			pet_id,
			uploaded_by_user_id,
			title,
			document_type,
			object_path,
			content_type,
			size_bytes
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		returning id::text
	`,
		tenantID,
		petID,
		uuidOrNil(actorUserID),
		strings.TrimSpace(input.Title),
		strings.TrimSpace(input.DocumentType),
		strings.TrimSpace(input.ObjectPath),
		strings.TrimSpace(input.ContentType),
		input.SizeBytes,
	).Scan(&documentID)
	if err != nil {
		return domain.PetDocumentMutationResult{}, fmt.Errorf("create pet document: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "pet_document.upload", "pet_document", documentID, input.Title); err != nil {
		return domain.PetDocumentMutationResult{}, err
	}

	document, err := petDocumentByID(ctx, tx, tenantID, documentID)
	if err != nil {
		return domain.PetDocumentMutationResult{}, err
	}
	result := domain.PetDocumentMutationResult{Document: document}
	if err := rememberPetDocumentIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.PetDocumentMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PetDocumentMutationResult{}, fmt.Errorf("commit pet document upload: %w", err)
	}
	return result, nil
}

func (s PostgresStore) CreatePetDocumentDownload(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, documentID string, idempotencyKey string) (domain.PetDocumentDownloadURLResult, error) {
	ownOnly, err := ownPetRecordReadOnly(actorRole)
	if err != nil && !roleHasAny(actorRole, domain.PermissionPetDocumentUpload) {
		return domain.PetDocumentDownloadURLResult{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PetDocumentDownloadURLResult{}, fmt.Errorf("begin pet document download url: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		PetID      string
		DocumentID string
	}{PetID: petID, DocumentID: documentID})
	if result, ok, err := idempotentPetDocumentDownloadURLResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	document, err := petDocumentByPetID(ctx, tx, tenantID, petID, documentID)
	if err != nil {
		return domain.PetDocumentDownloadURLResult{}, err
	}
	if ownOnly || (actorRole == domain.RolePetParent && !roleHasAny(actorRole, domain.PermissionPetRecordManage)) {
		if err := requirePetGuardianAccess(ctx, tx, tenantID, petID, actorUserID, true); err != nil {
			return domain.PetDocumentDownloadURLResult{}, err
		}
	}

	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	downloadURL, err := s.signedStorageURL(document.ObjectPath, "GET", expiresAt)
	if err != nil {
		return domain.PetDocumentDownloadURLResult{}, err
	}
	result := domain.PetDocumentDownloadURLResult{
		DocumentID:  documentID,
		ObjectPath:  document.ObjectPath,
		DownloadURL: downloadURL,
		Method:      "GET",
		ExpiresAt:   expiresAt.Format(time.RFC3339),
	}
	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "pet_document.download_url", "pet_document", documentID, document.Title); err != nil {
		return domain.PetDocumentDownloadURLResult{}, err
	}
	if err := rememberPetDocumentDownloadURLResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.PetDocumentDownloadURLResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.PetDocumentDownloadURLResult{}, fmt.Errorf("commit pet document download url: %w", err)
	}
	return result, nil
}

func (s PostgresStore) ArchivePetDocument(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, petID string, documentID string, input domain.ArchivePetDocumentInput, idempotencyKey string) (domain.PetDocumentMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPetRecordManage) {
		return domain.PetDocumentMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PetDocumentMutationResult{}, fmt.Errorf("begin pet document archive: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		PetID      string
		DocumentID string
		Input      domain.ArchivePetDocumentInput
	}{PetID: petID, DocumentID: documentID, Input: input})
	if result, ok, err := idempotentPetDocumentResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	document, err := petDocumentByPetID(ctx, tx, tenantID, petID, documentID)
	if err != nil {
		return domain.PetDocumentMutationResult{}, err
	}

	command, err := tx.Exec(ctx, `
		update pet_documents
		set status = 'archived',
			archived_at = now()
		where tenant_id = $1 and pet_id = $2 and id = $3 and archived_at is null
	`, tenantID, petID, documentID)
	if err != nil {
		return domain.PetDocumentMutationResult{}, fmt.Errorf("archive pet document: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.PetDocumentMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "pet_document.archive", "pet_document", documentID, input.Reason); err != nil {
		return domain.PetDocumentMutationResult{}, err
	}

	document.Status = "archived"
	result := domain.PetDocumentMutationResult{Document: document}
	if err := rememberPetDocumentIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.PetDocumentMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PetDocumentMutationResult{}, fmt.Errorf("commit pet document archive: %w", err)
	}
	return result, nil
}

func (s PostgresStore) PrescriptionTemplates(ctx context.Context, tenantID string) ([]domain.PrescriptionTemplate, error) {
	return s.demo.PrescriptionTemplates(ctx, tenantID)
}

func (s PostgresStore) Prescriptions(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.Prescription, error) {
	sharedOnly, err := sharedPrescriptionReadOnly(actorRole)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		select
			pr.id::text,
			p.name,
			coalesce(g.name, ''),
			pr.status::text,
			coalesce(string_agg(pm.medication_name, ', ' order by pm.medication_name), ''),
			coalesce(pr.instructions, ''),
			pr.shared_with_pet_parent,
			pr.updated_at
		from prescriptions pr
		join pets p on p.id = pr.pet_id and p.tenant_id = pr.tenant_id
		left join prescription_medications pm on pm.prescription_id = pr.id
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = pr.tenant_id and pet_id = pr.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where pr.tenant_id = $1
			and pr.archived_at is null
			and (
				$2::boolean = false
				or (
					pr.shared_with_pet_parent = true
					and exists (
						select 1
						from pet_guardians pg
						where pg.tenant_id = pr.tenant_id
							and pg.pet_id = pr.pet_id
							and pg.user_id = $3
							and pg.can_view_records = true
							and pg.archived_at is null
					)
				)
			)
		group by pr.id, p.name, g.name
		order by pr.updated_at desc
		limit 100
	`, tenantID, sharedOnly, uuidOrNil(actorUserID))
	if err != nil {
		return nil, fmt.Errorf("read prescriptions: %w", err)
	}
	defer rows.Close()

	items := []domain.Prescription{}
	for rows.Next() {
		var item domain.Prescription
		var medications string
		var updatedAt time.Time
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Status, &medications, &item.Instructions, &item.SharedWithPetParent, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan prescription: %w", err)
		}
		item.MedicationNames = splitNames(medications)
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prescriptions: %w", err)
	}
	return items, nil
}

func (s PostgresStore) CreatePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreatePrescriptionInput, idempotencyKey string) (domain.PrescriptionMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPrescriptionDraft) {
		return domain.PrescriptionMutationResult{}, domain.ErrForbidden
	}
	if !isUUID(actorUserID) {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("%w: actor user id must be a UUID", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("begin prescription create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentPrescriptionResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	prescribingVetID := input.PrescribingVeterinarianID
	if strings.TrimSpace(prescribingVetID) == "" && actorRole == domain.RoleVeterinarian {
		prescribingVetID = actorUserID
	}

	var prescriptionID string
	err = tx.QueryRow(ctx, `
		insert into prescriptions (
			tenant_id,
			location_id,
			pet_id,
			appointment_id,
			created_by_user_id,
			prescribing_veterinarian_user_id,
			status,
			instructions,
			shared_with_pet_parent
		)
		values ($1, $2, $3, $4, $5, $6, 'draft', $7, $8)
		returning id::text
	`,
		tenantID,
		input.LocationID,
		input.PetID,
		uuidOrNil(input.AppointmentID),
		actorUserID,
		uuidOrNil(prescribingVetID),
		nullString(input.Instructions),
		input.SharedWithPetParent,
	).Scan(&prescriptionID)
	if err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("create prescription: %w", err)
	}

	for _, medication := range input.Medications {
		if _, err := tx.Exec(ctx, `
			insert into prescription_medications (
				prescription_id,
				medication_name,
				strength,
				dosage,
				frequency,
				duration,
				route,
				instructions
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
			prescriptionID,
			strings.TrimSpace(medication.MedicationName),
			nullString(medication.Strength),
			nullString(medication.Dosage),
			nullString(medication.Frequency),
			nullString(medication.Duration),
			nullString(medication.Route),
			nullString(medication.Instructions),
		); err != nil {
			return domain.PrescriptionMutationResult{}, fmt.Errorf("create prescription medication: %w", err)
		}
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "prescription.create", "prescription", prescriptionID, input.Instructions); err != nil {
		return domain.PrescriptionMutationResult{}, err
	}

	prescription, err := prescriptionByID(ctx, tx, tenantID, prescriptionID)
	if err != nil {
		return domain.PrescriptionMutationResult{}, err
	}
	result := domain.PrescriptionMutationResult{Prescription: prescription}
	if err := rememberPrescriptionIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.PrescriptionMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("commit prescription create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) FinalizePrescription(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, prescriptionID string, input domain.FinalizePrescriptionInput, idempotencyKey string) (domain.PrescriptionMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPrescriptionFinalize) {
		return domain.PrescriptionMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("begin prescription finalize: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		PrescriptionID string
		Input          domain.FinalizePrescriptionInput
	}{PrescriptionID: prescriptionID, Input: input})
	if result, ok, err := idempotentPrescriptionResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var currentStatus string
	err = tx.QueryRow(ctx, `
		select status::text
		from prescriptions
		where tenant_id = $1 and id = $2 and archived_at is null
		for update
	`, tenantID, prescriptionID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PrescriptionMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("read prescription: %w", err)
	}
	if currentStatus == "finalized" {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("%w: prescription is already finalized", domain.ErrConflict)
	}

	command, err := tx.Exec(ctx, `
		update prescriptions
		set status = 'finalized',
			prescribing_veterinarian_user_id = coalesce(prescribing_veterinarian_user_id, $3),
			shared_with_pet_parent = shared_with_pet_parent or $4,
			finalized_at = now(),
			updated_at = now()
		where tenant_id = $1 and id = $2 and archived_at is null
	`, tenantID, prescriptionID, uuidOrNil(actorUserID), input.ShareWithPetParent)
	if err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("finalize prescription: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.PrescriptionMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "prescription.finalize", "prescription", prescriptionID, "finalized"); err != nil {
		return domain.PrescriptionMutationResult{}, err
	}

	prescription, err := prescriptionByID(ctx, tx, tenantID, prescriptionID)
	if err != nil {
		return domain.PrescriptionMutationResult{}, err
	}
	result := domain.PrescriptionMutationResult{Prescription: prescription}
	if err := rememberPrescriptionIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.PrescriptionMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.PrescriptionMutationResult{}, fmt.Errorf("commit prescription finalize: %w", err)
	}
	return result, nil
}

func (s PostgresStore) ClinicalNotes(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.ClinicalNote, error) {
	sharedOnly, err := sharedClinicalNoteReadOnly(actorRole)
	if err != nil {
		return nil, err
	}

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
		where n.tenant_id = $1
			and n.archived_at is null
			and (
				$2::boolean = false
				or (
					n.shared_with_pet_parent = true
					and exists (
						select 1
						from pet_guardians pg
						where pg.tenant_id = n.tenant_id
							and pg.pet_id = n.pet_id
							and pg.user_id = $3
							and pg.can_view_records = true
							and pg.archived_at is null
					)
				)
			)
		order by n.updated_at desc
		limit 100
	`, tenantID, sharedOnly, uuidOrNil(actorUserID))
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

func (s PostgresStore) CreateClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateClinicalNoteInput, idempotencyKey string) (domain.ClinicalNoteMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionClinicalNoteDraft) {
		return domain.ClinicalNoteMutationResult{}, domain.ErrForbidden
	}
	if !isUUID(actorUserID) {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("%w: actor user id must be a UUID", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("begin clinical note create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentClinicalNoteResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	vitals := "{}"
	if input.Vitals != nil {
		body, err := json.Marshal(input.Vitals)
		if err != nil {
			return domain.ClinicalNoteMutationResult{}, fmt.Errorf("%w: vitals must be JSON object compatible", domain.ErrValidation)
		}
		vitals = string(body)
	}

	var clinicalNoteID string
	err = tx.QueryRow(ctx, `
		insert into clinical_notes (
			tenant_id,
			location_id,
			pet_id,
			appointment_id,
			author_user_id,
			status,
			reason_for_visit,
			subjective,
			objective,
			assessment,
			plan,
			vitals,
			shared_with_pet_parent
		)
		values ($1, $2, $3, $4, $5, 'draft', $6, $7, $8, $9, $10, $11::jsonb, $12)
		returning id::text
	`,
		tenantID,
		input.LocationID,
		input.PetID,
		uuidOrNil(input.AppointmentID),
		actorUserID,
		nullString(input.ReasonForVisit),
		nullString(input.Subjective),
		nullString(input.Objective),
		nullString(input.Assessment),
		nullString(input.Plan),
		vitals,
		input.SharedWithPetParent,
	).Scan(&clinicalNoteID)
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("create clinical note: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "clinical_note.create", "clinical_note", clinicalNoteID, input.ReasonForVisit); err != nil {
		return domain.ClinicalNoteMutationResult{}, err
	}

	clinicalNote, err := clinicalNoteByID(ctx, tx, tenantID, clinicalNoteID)
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, err
	}
	result := domain.ClinicalNoteMutationResult{ClinicalNote: clinicalNote}
	if err := rememberClinicalNoteIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.ClinicalNoteMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("commit clinical note create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) FinalizeClinicalNote(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, clinicalNoteID string, input domain.FinalizeClinicalNoteInput, idempotencyKey string) (domain.ClinicalNoteMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionClinicalNoteFinalize) {
		return domain.ClinicalNoteMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("begin clinical note finalize: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		ClinicalNoteID string
		Input          domain.FinalizeClinicalNoteInput
	}{ClinicalNoteID: clinicalNoteID, Input: input})
	if result, ok, err := idempotentClinicalNoteResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var currentStatus string
	err = tx.QueryRow(ctx, `
		select status::text
		from clinical_notes
		where tenant_id = $1 and id = $2 and archived_at is null
		for update
	`, tenantID, clinicalNoteID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClinicalNoteMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("read clinical note: %w", err)
	}
	if currentStatus == "finalized" {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("%w: clinical note is already finalized", domain.ErrConflict)
	}

	command, err := tx.Exec(ctx, `
		update clinical_notes
		set status = 'finalized',
			finalized_by_user_id = $3,
			shared_with_pet_parent = shared_with_pet_parent or $4,
			finalized_at = now(),
			updated_at = now()
		where tenant_id = $1 and id = $2 and archived_at is null
	`, tenantID, clinicalNoteID, uuidOrNil(actorUserID), input.ShareWithPetParent)
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("finalize clinical note: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.ClinicalNoteMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "clinical_note.finalize", "clinical_note", clinicalNoteID, "finalized"); err != nil {
		return domain.ClinicalNoteMutationResult{}, err
	}

	clinicalNote, err := clinicalNoteByID(ctx, tx, tenantID, clinicalNoteID)
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, err
	}
	result := domain.ClinicalNoteMutationResult{ClinicalNote: clinicalNote}
	if err := rememberClinicalNoteIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.ClinicalNoteMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.ClinicalNoteMutationResult{}, fmt.Errorf("commit clinical note finalize: %w", err)
	}
	return result, nil
}

func (s PostgresStore) LabTests(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) ([]domain.LabTest, error) {
	sharedOnly, err := sharedLabResultReadOnly(actorRole)
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		select
			o.id::text,
			p.name,
			coalesce(g.name, ''),
			o.test_type,
			coalesce(c.name, 'Internal lab'),
			coalesce(c.lab_type::text, 'internal'),
			o.status::text,
			coalesce(r.report_object_path, ''),
			o.shared_with_pet_parent
		from lab_orders o
		join pets p on p.id = o.pet_id and p.tenant_id = o.tenant_id
		left join lab_centers c on c.id = o.lab_center_id and c.tenant_id = o.tenant_id
		left join lateral (
			select report_object_path
			from lab_results
			where tenant_id = o.tenant_id and lab_order_id = o.id
			order by created_at desc
			limit 1
		) r on true
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = o.tenant_id and pet_id = o.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where o.tenant_id = $1
			and o.cancelled_at is null
			and (
				$2::boolean = false
				or (
					o.shared_with_pet_parent = true
					and exists (
						select 1
						from pet_guardians pg
						where pg.tenant_id = o.tenant_id
							and pg.pet_id = o.pet_id
							and pg.user_id = $3
							and pg.can_view_records = true
							and pg.archived_at is null
					)
				)
			)
		order by o.created_at desc
		limit 100
	`, tenantID, sharedOnly, uuidOrNil(actorUserID))
	if err != nil {
		return nil, fmt.Errorf("read lab tests: %w", err)
	}
	defer rows.Close()

	items := []domain.LabTest{}
	for rows.Next() {
		var item domain.LabTest
		var status string
		if err := rows.Scan(&item.ID, &item.PetName, &item.OwnerName, &item.TestType, &item.LabCenter, &item.LabType, &status, &item.ReportURL, &item.SharedWithPetParent); err != nil {
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

func (s PostgresStore) CreateLabOrder(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateLabOrderInput, idempotencyKey string) (domain.LabOrderMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionLabOrderCreate) {
		return domain.LabOrderMutationResult{}, domain.ErrForbidden
	}
	if !isUUID(actorUserID) {
		return domain.LabOrderMutationResult{}, fmt.Errorf("%w: actor user id must be a UUID", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("begin lab order create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentLabOrderResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	priority := strings.TrimSpace(input.Priority)
	if priority == "" {
		priority = "normal"
	}

	var labOrderID string
	err = tx.QueryRow(ctx, `
		insert into lab_orders (
			tenant_id,
			location_id,
			pet_id,
			appointment_id,
			lab_center_id,
			ordered_by_user_id,
			test_type,
			sample_type,
			priority
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning id::text
	`,
		tenantID,
		input.LocationID,
		input.PetID,
		uuidOrNil(input.AppointmentID),
		uuidOrNil(input.LabCenterID),
		actorUserID,
		strings.TrimSpace(input.TestType),
		nullString(input.SampleType),
		priority,
	).Scan(&labOrderID)
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("create lab order: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "lab_order.create", "lab_order", labOrderID, input.TestType); err != nil {
		return domain.LabOrderMutationResult{}, err
	}

	labTest, err := labTestByID(ctx, tx, tenantID, labOrderID)
	if err != nil {
		return domain.LabOrderMutationResult{}, err
	}
	result := domain.LabOrderMutationResult{LabTest: labTest}
	if err := rememberLabOrderIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.LabOrderMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("commit lab order create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) UpdateLabOrderStatus(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, labOrderID string, input domain.UpdateLabOrderStatusInput, idempotencyKey string) (domain.LabOrderMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionLabOrderProcess) {
		return domain.LabOrderMutationResult{}, domain.ErrForbidden
	}
	if !validLabOrderStatus(input.Status) || input.Status == domain.LabOrdered {
		return domain.LabOrderMutationResult{}, fmt.Errorf("%w: unsupported lab order status", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("begin lab order status update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		LabOrderID string
		Input      domain.UpdateLabOrderStatusInput
	}{LabOrderID: labOrderID, Input: input})
	if result, ok, err := idempotentLabOrderResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var currentStatus string
	err = tx.QueryRow(ctx, `
		select status::text
		from lab_orders
		where tenant_id = $1 and id = $2
		for update
	`, tenantID, labOrderID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LabOrderMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("read lab order: %w", err)
	}

	if !labOrderTransitionAllowed(domain.LabOrderStatus(currentStatus), input.Status) {
		return domain.LabOrderMutationResult{}, fmt.Errorf("%w: lab order cannot move from %s to %s", domain.ErrConflict, currentStatus, input.Status)
	}

	command, err := tx.Exec(ctx, `
		update lab_orders
		set status = $3::lab_order_status,
			cancelled_at = case when $3::lab_order_status = 'cancelled' then now() else cancelled_at end,
			updated_at = now()
		where tenant_id = $1 and id = $2
	`, tenantID, labOrderID, string(input.Status))
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("update lab order status: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.LabOrderMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "lab_order."+string(input.Status), "lab_order", labOrderID, input.Reason); err != nil {
		return domain.LabOrderMutationResult{}, err
	}

	labTest, err := labTestByID(ctx, tx, tenantID, labOrderID)
	if err != nil {
		return domain.LabOrderMutationResult{}, err
	}
	result := domain.LabOrderMutationResult{LabTest: labTest}
	if err := rememberLabOrderIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.LabOrderMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("commit lab order status update: %w", err)
	}
	return result, nil
}

func (s PostgresStore) UploadLabResult(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, labOrderID string, input domain.UploadLabResultInput, idempotencyKey string) (domain.LabOrderMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionLabOrderProcess, domain.PermissionLabResultShare) {
		return domain.LabOrderMutationResult{}, domain.ErrForbidden
	}
	if input.ShareWithPetParent && !roleHasAny(actorRole, domain.PermissionLabResultShare) {
		return domain.LabOrderMutationResult{}, domain.ErrForbidden
	}

	completedAt, err := parseOptionalTime(input.CompletedAt)
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("%w: completedAt must be RFC3339", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("begin lab result upload: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		LabOrderID string
		Input      domain.UploadLabResultInput
	}{LabOrderID: labOrderID, Input: input})
	if result, ok, err := idempotentLabOrderResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var currentStatus string
	err = tx.QueryRow(ctx, `
		select status::text
		from lab_orders
		where tenant_id = $1 and id = $2
		for update
	`, tenantID, labOrderID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LabOrderMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("read lab order: %w", err)
	}
	if currentStatus == string(domain.LabCancelled) {
		return domain.LabOrderMutationResult{}, fmt.Errorf("%w: cancelled lab orders cannot receive results", domain.ErrConflict)
	}

	var labResultID string
	err = tx.QueryRow(ctx, `
		insert into lab_results (
			tenant_id,
			lab_order_id,
			uploaded_by_user_id,
			result_notes,
			report_object_path,
			completed_at
		)
		values (
			$1,
			$2,
			$3,
			$4,
			$5,
			case when $6 then coalesce($7, now()) else $7 end
		)
		returning id::text
	`,
		tenantID,
		labOrderID,
		uuidOrNil(actorUserID),
		nullString(input.ResultNotes),
		nullString(input.ReportObjectPath),
		input.MarkOrderCompleted,
		completedAt,
	).Scan(&labResultID)
	if err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("create lab result: %w", err)
	}

	nextStatus := domain.LabInProgress
	if input.MarkOrderCompleted {
		nextStatus = domain.LabCompleted
	}
	if currentStatus == string(domain.LabCompleted) && !input.MarkOrderCompleted {
		nextStatus = domain.LabCompleted
	}

	if _, err := tx.Exec(ctx, `
		update lab_orders
		set status = $3::lab_order_status,
			shared_with_pet_parent = shared_with_pet_parent or $4,
			updated_at = now()
		where tenant_id = $1 and id = $2
	`, tenantID, labOrderID, string(nextStatus), input.ShareWithPetParent); err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("update lab order after result upload: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "lab_result.upload", "lab_result", labResultID, input.ResultNotes); err != nil {
		return domain.LabOrderMutationResult{}, err
	}

	labTest, err := labTestByID(ctx, tx, tenantID, labOrderID)
	if err != nil {
		return domain.LabOrderMutationResult{}, err
	}
	result := domain.LabOrderMutationResult{LabTest: labTest}
	if err := rememberLabOrderIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.LabOrderMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.LabOrderMutationResult{}, fmt.Errorf("commit lab result upload: %w", err)
	}
	return result, nil
}

func (s PostgresStore) Billing(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role) (map[string]any, error) {
	ownOnly, err := ownInvoiceReadOnly(actorRole)
	if err != nil {
		return nil, err
	}
	actorID := uuidOrNil(actorUserID)

	today, err := s.sumCents(ctx, `
		select coalesce(sum(total_cents), 0)
		from invoices i
		where i.tenant_id = $1
			and i.status = 'paid'
			and i.paid_at::date = current_date
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = i.tenant_id
						and pg.pet_id = i.pet_id
						and pg.user_id = $3
						and pg.archived_at is null
				)
			)
	`, tenantID, ownOnly, actorID)
	if err != nil {
		return nil, err
	}
	pending, err := s.sumCents(ctx, `
		select coalesce(sum(total_cents), 0)
		from invoices i
		where i.tenant_id = $1
			and i.status in ('issued', 'pending')
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = i.tenant_id
						and pg.pet_id = i.pet_id
						and pg.user_id = $3
						and pg.archived_at is null
				)
			)
	`, tenantID, ownOnly, actorID)
	if err != nil {
		return nil, err
	}
	allTime, err := s.sumCents(ctx, `
		select coalesce(sum(total_cents), 0)
		from invoices i
		where i.tenant_id = $1
			and i.status = 'paid'
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = i.tenant_id
						and pg.pet_id = i.pet_id
						and pg.user_id = $3
						and pg.archived_at is null
				)
			)
	`, tenantID, ownOnly, actorID)
	if err != nil {
		return nil, err
	}
	overdue, err := s.count(ctx, `
		select count(*)
		from invoices i
		where i.tenant_id = $1
			and i.status in ('issued', 'pending')
			and i.due_at < now() - interval '30 days'
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = i.tenant_id
						and pg.pet_id = i.pet_id
						and pg.user_id = $3
						and pg.archived_at is null
				)
			)
	`, tenantID, ownOnly, actorID)
	if err != nil {
		return nil, err
	}
	invoices, err := s.invoices(ctx, tenantID, ownOnly, actorID)
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

func (s PostgresStore) CreateInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateInvoiceInput, idempotencyKey string) (domain.InvoiceMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionInvoiceCreate) {
		return domain.InvoiceMutationResult{}, domain.ErrForbidden
	}

	dueAt, err := parseOptionalTime(input.DueAt)
	if err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: dueAt must be RFC3339", domain.ErrValidation)
	}

	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "issued"
	}
	if status != "draft" && status != "issued" {
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: status must be draft or issued", domain.ErrValidation)
	}
	if input.TaxCents < 0 || input.DiscountCents < 0 {
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: invoice adjustments must be non-negative", domain.ErrValidation)
	}
	if len(input.LineItems) == 0 {
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: lineItems is required", domain.ErrValidation)
	}

	subtotal := int64(0)
	for _, line := range input.LineItems {
		if strings.TrimSpace(line.Description) == "" || line.Quantity <= 0 || line.UnitAmountCents < 0 {
			return domain.InvoiceMutationResult{}, fmt.Errorf("%w: invalid invoice line item", domain.ErrValidation)
		}
		subtotal += int64(line.Quantity) * line.UnitAmountCents
	}
	total := subtotal + input.TaxCents - input.DiscountCents
	if total < 0 {
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: discount cannot exceed subtotal plus tax", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("begin invoice create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentInvoiceResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var invoiceID string
	err = tx.QueryRow(ctx, `
		insert into invoices (
			tenant_id,
			location_id,
			pet_id,
			status,
			subtotal_cents,
			tax_cents,
			discount_cents,
			total_cents,
			due_at
		)
		values ($1, $2, $3, $4::invoice_status, $5, $6, $7, $8, $9)
		returning id::text
	`,
		tenantID,
		input.LocationID,
		uuidOrNil(input.PetID),
		status,
		subtotal,
		input.TaxCents,
		input.DiscountCents,
		total,
		dueAt,
	).Scan(&invoiceID)
	if err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("create invoice: %w", err)
	}

	for _, line := range input.LineItems {
		lineTotal := int64(line.Quantity) * line.UnitAmountCents
		if _, err := tx.Exec(ctx, `
			insert into invoice_line_items (
				invoice_id,
				tenant_id,
				description,
				quantity,
				unit_amount_cents,
				total_amount_cents,
				related_resource_type,
				related_resource_id
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
			invoiceID,
			tenantID,
			strings.TrimSpace(line.Description),
			line.Quantity,
			line.UnitAmountCents,
			lineTotal,
			nullString(line.RelatedResourceType),
			uuidOrNil(line.RelatedResourceID),
		); err != nil {
			return domain.InvoiceMutationResult{}, fmt.Errorf("create invoice line item: %w", err)
		}
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "invoice.create", "invoice", invoiceID, status); err != nil {
		return domain.InvoiceMutationResult{}, err
	}

	invoice, err := invoiceByID(ctx, tx, tenantID, invoiceID)
	if err != nil {
		return domain.InvoiceMutationResult{}, err
	}
	result := domain.InvoiceMutationResult{Invoice: invoice}
	if err := rememberInvoiceIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.InvoiceMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("commit invoice create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) VoidInvoice(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, invoiceID string, input domain.VoidInvoiceInput, idempotencyKey string) (domain.InvoiceMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionPaymentRefundVoid) {
		return domain.InvoiceMutationResult{}, domain.ErrForbidden
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("begin invoice void: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(struct {
		InvoiceID string
		Input     domain.VoidInvoiceInput
	}{InvoiceID: invoiceID, Input: input})
	if result, ok, err := idempotentInvoiceResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	var status string
	err = tx.QueryRow(ctx, `
		select status::text
		from invoices
		where tenant_id = $1 and id = $2
		for update
	`, tenantID, invoiceID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InvoiceMutationResult{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("read invoice: %w", err)
	}
	switch status {
	case "draft", "issued", "pending":
	case "void":
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: invoice is already void", domain.ErrConflict)
	default:
		return domain.InvoiceMutationResult{}, fmt.Errorf("%w: %s invoices cannot be voided", domain.ErrConflict, status)
	}

	command, err := tx.Exec(ctx, `
		update invoices
		set status = 'void',
			voided_at = now(),
			updated_at = now()
		where tenant_id = $1 and id = $2
	`, tenantID, invoiceID)
	if err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("void invoice: %w", err)
	}
	if command.RowsAffected() == 0 {
		return domain.InvoiceMutationResult{}, domain.ErrNotFound
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "invoice.void", "invoice", invoiceID, input.Reason); err != nil {
		return domain.InvoiceMutationResult{}, err
	}

	invoice, err := invoiceByID(ctx, tx, tenantID, invoiceID)
	if err != nil {
		return domain.InvoiceMutationResult{}, err
	}
	result := domain.InvoiceMutationResult{Invoice: invoice}
	if err := rememberInvoiceIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusOK, result); err != nil {
		return domain.InvoiceMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.InvoiceMutationResult{}, fmt.Errorf("commit invoice void: %w", err)
	}
	return result, nil
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

func (s PostgresStore) CreateStaff(ctx context.Context, tenantID string, actorUserID string, actorRole domain.Role, input domain.CreateStaffInput, idempotencyKey string) (domain.StaffMutationResult, error) {
	if !roleHasAny(actorRole, domain.PermissionStaffManage) {
		return domain.StaffMutationResult{}, domain.ErrForbidden
	}
	if !validManagedStaffRole(input.Role) {
		return domain.StaffMutationResult{}, fmt.Errorf("%w: role is not supported for staff management", domain.ErrValidation)
	}
	if strings.TrimSpace(input.DefaultLocationID) != "" && !isUUID(input.DefaultLocationID) {
		return domain.StaffMutationResult{}, fmt.Errorf("%w: defaultLocationId must be a UUID", domain.ErrValidation)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.StaffMutationResult{}, fmt.Errorf("begin staff create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(input)
	if result, ok, err := idempotentStaffResult(ctx, tx, tenantID, idempotencyKey, hash); ok || err != nil {
		return result, err
	}

	email := strings.ToLower(strings.TrimSpace(input.Email))
	name := strings.TrimSpace(input.Name)
	roleCode := string(input.Role)

	var roleID string
	err = tx.QueryRow(ctx, "select id::text from roles where code = $1", roleCode).Scan(&roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StaffMutationResult{}, fmt.Errorf("%w: role is not configured", domain.ErrValidation)
	}
	if err != nil {
		return domain.StaffMutationResult{}, fmt.Errorf("read staff role: %w", err)
	}

	var userID string
	err = tx.QueryRow(ctx, `
		insert into users (email, password_hash, display_name, status)
		values ($1, 'invited:pending', $2, 'invited')
		on conflict (email_normalized) do update
		set display_name = excluded.display_name,
			archived_at = null,
			updated_at = now()
		returning id::text
	`, email, name).Scan(&userID)
	if err != nil {
		return domain.StaffMutationResult{}, fmt.Errorf("create staff user: %w", err)
	}

	var membershipID string
	err = tx.QueryRow(ctx, `
		insert into tenant_memberships (tenant_id, user_id, default_location_id, status)
		values ($1, $2, $3, 'invited')
		on conflict (tenant_id, user_id) do update
		set default_location_id = coalesce(excluded.default_location_id, tenant_memberships.default_location_id),
			status = case
				when tenant_memberships.status = 'archived' then 'invited'::user_status
				else tenant_memberships.status
			end,
			archived_at = null,
			updated_at = now()
		returning id::text
	`, tenantID, userID, uuidOrNil(input.DefaultLocationID)).Scan(&membershipID)
	if err != nil {
		return domain.StaffMutationResult{}, fmt.Errorf("create staff membership: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into membership_roles (membership_id, role_id)
		values ($1, $2)
		on conflict do nothing
	`, membershipID, roleID); err != nil {
		return domain.StaffMutationResult{}, fmt.Errorf("assign staff role: %w", err)
	}

	if err := audit(ctx, tx, tenantID, actorUserID, actorRole, "staff.create", "user", userID, roleCode); err != nil {
		return domain.StaffMutationResult{}, err
	}

	staffMember, err := staffMemberByID(ctx, tx, tenantID, userID, roleCode)
	if err != nil {
		return domain.StaffMutationResult{}, err
	}
	result := domain.StaffMutationResult{StaffMember: staffMember}
	if err := rememberStaffIdempotentResult(ctx, tx, tenantID, idempotencyKey, hash, httpStatusCreated, result); err != nil {
		return domain.StaffMutationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.StaffMutationResult{}, fmt.Errorf("commit staff create: %w", err)
	}
	return result, nil
}

func (s PostgresStore) AuditLogs(ctx context.Context, tenantID string) ([]domain.AuditLogEntry, error) {
	rows, err := s.pool.Query(ctx, `
		select
			id::text,
			coalesce(actor_user_id::text, ''),
			coalesce(actor_role, ''),
			action,
			resource_type,
			coalesce(resource_id::text, ''),
			coalesce(reason, ''),
			created_at
		from audit_logs
		where tenant_id = $1
		order by created_at desc
		limit 100
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("read audit logs: %w", err)
	}
	defer rows.Close()

	items := []domain.AuditLogEntry{}
	for rows.Next() {
		var item domain.AuditLogEntry
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.ActorUserID, &item.ActorRole, &item.Action, &item.ResourceType, &item.ResourceID, &item.Reason, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}
	return items, nil
}

func (s PostgresStore) count(ctx context.Context, query string, args ...any) (int64, error) {
	var value int64
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&value); err != nil {
		return 0, fmt.Errorf("read count: %w", err)
	}
	return value, nil
}

func (s PostgresStore) sumCents(ctx context.Context, query string, args ...any) (int64, error) {
	var value int64
	if err := s.pool.QueryRow(ctx, query, args...).Scan(&value); err != nil {
		return 0, fmt.Errorf("read amount: %w", err)
	}
	return value, nil
}

func (s PostgresStore) invoices(ctx context.Context, tenantID string, ownOnly bool, actorUserID any) ([]domain.Invoice, error) {
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
			and (
				$2::boolean = false
				or exists (
					select 1
					from pet_guardians pg
					where pg.tenant_id = i.tenant_id
						and pg.pet_id = i.pet_id
						and pg.user_id = $3
						and pg.archived_at is null
				)
			)
		order by i.created_at desc
		limit 100
	`, tenantID, ownOnly, actorUserID)
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

func invoiceByID(ctx context.Context, q rowQuerier, tenantID string, invoiceID string) (domain.Invoice, error) {
	var item domain.Invoice
	var dueAt sql.NullTime
	err := q.QueryRow(ctx, `
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
		where i.tenant_id = $1 and i.id = $2
	`, tenantID, invoiceID).Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Amount, &item.Status, &dueAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Invoice{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Invoice{}, fmt.Errorf("read invoice: %w", err)
	}
	if dueAt.Valid {
		item.DueDate = dueAt.Time.UTC().Format(time.DateOnly)
	}
	return item, nil
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

func staffMemberByID(ctx context.Context, q rowQuerier, tenantID string, userID string, roleCode string) (domain.Person, error) {
	var item domain.Person
	err := q.QueryRow(ctx, `
		select
			u.id::text,
			u.display_name,
			r.code,
			case when r.code = 'Veterinarian' then 'Small Animal Medicine' else '' end,
			u.email,
			m.status::text
		from tenant_memberships m
		join users u on u.id = m.user_id
		join membership_roles mr on mr.membership_id = m.id
		join roles r on r.id = mr.role_id
		where m.tenant_id = $1
			and u.id = $2
			and r.code = $3
			and m.archived_at is null
	`, tenantID, userID, roleCode).Scan(&item.ID, &item.Name, &item.Role, &item.Specialty, &item.Email, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Person{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Person{}, fmt.Errorf("read staff member: %w", err)
	}
	return item, nil
}

func prescriptionByID(ctx context.Context, q rowQuerier, tenantID string, prescriptionID string) (domain.Prescription, error) {
	var item domain.Prescription
	var medications string
	var updatedAt time.Time
	err := q.QueryRow(ctx, `
		select
			pr.id::text,
			p.name,
			coalesce(g.name, ''),
			pr.status::text,
			coalesce(string_agg(pm.medication_name, ', ' order by pm.medication_name), ''),
			coalesce(pr.instructions, ''),
			pr.shared_with_pet_parent,
			pr.updated_at
		from prescriptions pr
		join pets p on p.id = pr.pet_id and p.tenant_id = pr.tenant_id
		left join prescription_medications pm on pm.prescription_id = pr.id
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = pr.tenant_id and pet_id = pr.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where pr.tenant_id = $1 and pr.id = $2 and pr.archived_at is null
		group by pr.id, p.name, g.name
	`, tenantID, prescriptionID).Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Status, &medications, &item.Instructions, &item.SharedWithPetParent, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Prescription{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Prescription{}, fmt.Errorf("read prescription: %w", err)
	}
	item.MedicationNames = splitNames(medications)
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return item, nil
}

func clinicalNoteByID(ctx context.Context, q rowQuerier, tenantID string, clinicalNoteID string) (domain.ClinicalNote, error) {
	var item domain.ClinicalNote
	var updatedAt time.Time
	err := q.QueryRow(ctx, `
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
		where n.tenant_id = $1 and n.id = $2 and n.archived_at is null
	`, tenantID, clinicalNoteID).Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Subject, &item.Status, &updatedAt, &item.SharedWithPetParent)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClinicalNote{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ClinicalNote{}, fmt.Errorf("read clinical note: %w", err)
	}
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return item, nil
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

func documentObjectPath(tenantID string, petID string, title string, contentType string) string {
	stamp := time.Now().UTC().Format("20060102T150405000000000Z")
	name := safeObjectName(title)
	extension := extensionForContentType(contentType)
	if extension != "" && !strings.HasSuffix(name, extension) {
		name += extension
	}
	return fmt.Sprintf("tenants/%s/pets/%s/documents/%s-%s", strings.TrimSpace(tenantID), strings.TrimSpace(petID), stamp, name)
}

func safeObjectName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, ch := range value {
		allowed := (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '.'
		if allowed {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(b.String(), "-.")
	if name == "" {
		return "document"
	}
	if len(name) > 80 {
		name = name[:80]
		name = strings.Trim(name, "-.")
	}
	if name == "" {
		return "document"
	}
	return name
}

func extensionForContentType(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/pdf":
		return ".pdf"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "text/plain":
		return ".txt"
	default:
		return ""
	}
}

func (s PostgresStore) signedStorageURL(objectPath string, method string, expiresAt time.Time) (string, error) {
	bucket := strings.TrimSpace(s.documentBucket)
	if bucket == "" {
		return "", fmt.Errorf("document bucket is not configured")
	}
	if s.signingEmail == "" || s.privateKeyPEM == "" {
		return devStorageURL(bucket, objectPath, method, expiresAt), nil
	}

	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	timestamp := now.Format("20060102T150405Z")
	scope := datestamp + "/auto/storage/goog4_request"
	credential := s.signingEmail + "/" + scope
	signedHeaders := "host"
	canonicalURI := "/" + url.PathEscape(bucket) + "/" + strings.ReplaceAll(url.PathEscape(strings.TrimLeft(objectPath, "/")), "%2F", "/")

	query := url.Values{}
	query.Set("X-Goog-Algorithm", "GOOG4-RSA-SHA256")
	query.Set("X-Goog-Credential", credential)
	query.Set("X-Goog-Date", timestamp)
	query.Set("X-Goog-Expires", strconv.FormatInt(int64(time.Until(expiresAt).Seconds()), 10))
	query.Set("X-Goog-SignedHeaders", signedHeaders)

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		query.Encode(),
		"host:storage.googleapis.com\n",
		signedHeaders,
		"UNSIGNED-PAYLOAD",
	}, "\n")
	requestHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"GOOG4-RSA-SHA256",
		timestamp,
		scope,
		hex.EncodeToString(requestHash[:]),
	}, "\n")

	signature, err := signRSASHA256(s.privateKeyPEM, stringToSign)
	if err != nil {
		return "", err
	}
	query.Set("X-Goog-Signature", signature)
	return "https://storage.googleapis.com" + canonicalURI + "?" + query.Encode(), nil
}

func devStorageURL(bucket string, objectPath string, method string, expiresAt time.Time) string {
	values := url.Values{}
	values.Set("X-Goog-Algorithm", "GOOG4-RSA-SHA256")
	values.Set("X-Goog-Credential", "pawit-v1")
	values.Set("X-Goog-Date", time.Now().UTC().Format("20060102T150405Z"))
	values.Set("X-Goog-Expires", strconv.FormatInt(int64(time.Until(expiresAt).Seconds()), 10))
	values.Set("X-Goog-SignedHeaders", "host")
	values.Set("X-Goog-Method", method)
	values.Set("X-Goog-Signature", mutationHash(struct {
		Bucket string
		Object string
		Method string
		Expiry string
	}{Bucket: bucket, Object: objectPath, Method: method, Expiry: expiresAt.Format(time.RFC3339)}))
	escapedObject := strings.ReplaceAll(url.PathEscape(strings.TrimLeft(objectPath, "/")), "%2F", "/")
	return "https://storage.googleapis.com/" + url.PathEscape(strings.TrimSpace(bucket)) + "/" + escapedObject + "?" + values.Encode()
}

func signRSASHA256(privateKeyPEM string, value string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("decode GCS private key PEM: no PEM block found")
	}
	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("decode GCS private key PEM: key is not RSA")
		}
		key = rsaKey
	} else {
		parsed, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
		if parseErr != nil {
			return "", fmt.Errorf("decode GCS private key PEM: %w", err)
		}
		key = parsed
	}
	digest := sha256.Sum256([]byte(value))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign GCS URL: %w", err)
	}
	return hex.EncodeToString(signature), nil
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type queryer interface {
	rowQuerier
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func tenantByID(ctx context.Context, q queryer, tenantID string) (domain.Tenant, error) {
	var item domain.Tenant
	var createdAt, updatedAt time.Time
	err := q.QueryRow(ctx, `
		select
			id::text,
			name,
			coalesce(legal_name, ''),
			status::text,
			coalesce(stripe_customer_id, ''),
			default_cancellation_cutoff_hours,
			created_at,
			updated_at
		from tenants
		where id = $1
	`, tenantID).Scan(&item.ID, &item.Name, &item.LegalName, &item.Status, &item.StripeCustomerID, &item.DefaultCancellationCutoffHours, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Tenant{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Tenant{}, fmt.Errorf("read tenant: %w", err)
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	locations, err := locationsForTenant(ctx, q, tenantID, false)
	if err != nil {
		return domain.Tenant{}, err
	}
	item.Locations = locations
	return item, nil
}

func locationsForTenant(ctx context.Context, q queryer, tenantID string, activeOnly bool) ([]domain.ClinicLocation, error) {
	rows, err := q.Query(ctx, `
		select
			id::text,
			name,
			timezone,
			coalesce(phone, ''),
			coalesce(email, ''),
			status::text
		from clinic_locations
		where tenant_id = $1
			and ($2::boolean = false or (archived_at is null and status = 'active'))
		order by name
	`, tenantID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("read tenant locations: %w", err)
	}
	defer rows.Close()

	items := []domain.ClinicLocation{}
	for rows.Next() {
		var item domain.ClinicLocation
		if err := rows.Scan(&item.ID, &item.Name, &item.Timezone, &item.Phone, &item.Email, &item.Status); err != nil {
			return nil, fmt.Errorf("scan tenant location: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tenant locations: %w", err)
	}
	return items, nil
}

func clinicLocationByID(ctx context.Context, q rowQuerier, tenantID string, locationID string) (domain.ClinicLocation, error) {
	var item domain.ClinicLocation
	err := q.QueryRow(ctx, `
		select
			id::text,
			name,
			timezone,
			coalesce(phone, ''),
			coalesce(email, ''),
			status::text
		from clinic_locations
		where tenant_id = $1 and id = $2
	`, tenantID, locationID).Scan(&item.ID, &item.Name, &item.Timezone, &item.Phone, &item.Email, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClinicLocation{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ClinicLocation{}, fmt.Errorf("read clinic location: %w", err)
	}
	return item, nil
}

func appointmentByID(ctx context.Context, q rowQuerier, tenantID string, appointmentID string) (domain.Appointment, error) {
	var item domain.Appointment
	var appointmentType, status, additional string
	err := q.QueryRow(ctx, `
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
			coalesce(a.cancellation_reason, a.reason)
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
		where a.tenant_id = $1 and a.id = $2 and a.archived_at is null
	`, tenantID, appointmentID).Scan(
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
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Appointment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Appointment{}, fmt.Errorf("read appointment: %w", err)
	}
	item.Type = domain.AppointmentType(appointmentType)
	item.Status = domain.AppointmentStatus(status)
	item.AdditionalVeterinarians = splitNames(additional)
	return item, nil
}

func queueEntryByID(ctx context.Context, q rowQuerier, tenantID string, queueID string) (domain.QueueEntry, error) {
	var item domain.QueueEntry
	var status string
	err := q.QueryRow(ctx, `
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
		where q.tenant_id = $1 and q.id = $2
	`, tenantID, queueID).Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Species, &item.Priority, &status, &item.WaitMins)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueueEntry{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.QueueEntry{}, fmt.Errorf("read queue entry: %w", err)
	}
	item.Status = domain.QueueStatus(status)
	return item, nil
}

func patientByID(ctx context.Context, q rowQuerier, tenantID string, petID string) (domain.PatientRecord, error) {
	var item domain.PatientRecord
	err := q.QueryRow(ctx, `
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
		where p.tenant_id = $1 and p.id = $2 and p.archived_at is null
	`, tenantID, petID).Scan(&item.ID, &item.PetName, &item.OwnerName, &item.Species, &item.Breed, &item.Age, &item.Sex, &item.Phone, &item.LastVisit, &item.GuardianCount, &item.DocumentsCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PatientRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PatientRecord{}, fmt.Errorf("read pet record: %w", err)
	}
	return item, nil
}

func petDocumentByID(ctx context.Context, q rowQuerier, tenantID string, documentID string) (domain.PetDocument, error) {
	var item domain.PetDocument
	var createdAt time.Time
	err := q.QueryRow(ctx, `
		select
			id::text,
			pet_id::text,
			title,
			document_type,
			object_path,
			content_type,
			size_bytes,
			status::text,
			created_at
		from pet_documents
		where tenant_id = $1 and id = $2 and archived_at is null
	`, tenantID, documentID).Scan(&item.ID, &item.PetID, &item.Title, &item.DocumentType, &item.ObjectPath, &item.ContentType, &item.SizeBytes, &item.Status, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetDocument{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PetDocument{}, fmt.Errorf("read pet document: %w", err)
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return item, nil
}

func petDocumentByPetID(ctx context.Context, q rowQuerier, tenantID string, petID string, documentID string) (domain.PetDocument, error) {
	var item domain.PetDocument
	var createdAt time.Time
	err := q.QueryRow(ctx, `
		select
			id::text,
			pet_id::text,
			title,
			document_type,
			object_path,
			content_type,
			size_bytes,
			status::text,
			created_at
		from pet_documents
		where tenant_id = $1 and pet_id = $2 and id = $3 and archived_at is null
	`, tenantID, petID, documentID).Scan(&item.ID, &item.PetID, &item.Title, &item.DocumentType, &item.ObjectPath, &item.ContentType, &item.SizeBytes, &item.Status, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetDocument{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.PetDocument{}, fmt.Errorf("read pet document: %w", err)
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return item, nil
}

func labTestByID(ctx context.Context, q rowQuerier, tenantID string, labOrderID string) (domain.LabTest, error) {
	var item domain.LabTest
	var status string
	err := q.QueryRow(ctx, `
		select
			o.id::text,
			p.name,
			coalesce(g.name, ''),
			o.test_type,
			coalesce(c.name, 'Internal lab'),
			coalesce(c.lab_type::text, 'internal'),
			o.status::text,
			coalesce(r.report_object_path, ''),
			o.shared_with_pet_parent
		from lab_orders o
		join pets p on p.id = o.pet_id and p.tenant_id = o.tenant_id
		left join lab_centers c on c.id = o.lab_center_id and c.tenant_id = o.tenant_id
		left join lateral (
			select report_object_path
			from lab_results
			where tenant_id = o.tenant_id and lab_order_id = o.id
			order by created_at desc
			limit 1
		) r on true
		left join lateral (
			select name
			from pet_guardians
			where tenant_id = o.tenant_id and pet_id = o.pet_id and archived_at is null
			order by is_primary desc, created_at
			limit 1
		) g on true
		where o.tenant_id = $1 and o.id = $2
	`, tenantID, labOrderID).Scan(&item.ID, &item.PetName, &item.OwnerName, &item.TestType, &item.LabCenter, &item.LabType, &status, &item.ReportURL, &item.SharedWithPetParent)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LabTest{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.LabTest{}, fmt.Errorf("read lab order: %w", err)
	}
	item.Status = domain.LabOrderStatus(status)
	return item, nil
}

func roleHasAny(role domain.Role, permissions ...domain.Permission) bool {
	for _, policy := range domain.PawItRolePolicies() {
		if policy.Role != role {
			continue
		}
		for _, granted := range policy.Permissions {
			for _, required := range permissions {
				if granted == required {
					return true
				}
			}
		}
	}
	return false
}

func ownAppointmentReadOnly(role domain.Role) (bool, error) {
	if roleHasAny(role, domain.PermissionAppointmentManage) {
		return false, nil
	}
	if roleHasAny(role, domain.PermissionAppointmentRequestOwn) {
		return true, nil
	}
	return false, domain.ErrForbidden
}

func sharedPrescriptionReadOnly(role domain.Role) (bool, error) {
	if roleHasAny(role, domain.PermissionPrescriptionView) {
		return false, nil
	}
	if roleHasAny(role, domain.PermissionPrescriptionViewShared) {
		return true, nil
	}
	return false, domain.ErrForbidden
}

func sharedClinicalNoteReadOnly(role domain.Role) (bool, error) {
	if roleHasAny(role, domain.PermissionClinicalNoteView) {
		return false, nil
	}
	if roleHasAny(role, domain.PermissionClinicalNoteViewShared) {
		return true, nil
	}
	return false, domain.ErrForbidden
}

func sharedLabResultReadOnly(role domain.Role) (bool, error) {
	if roleHasAny(role, domain.PermissionLabOrderCreate, domain.PermissionLabOrderProcess, domain.PermissionLabResultShare) {
		return false, nil
	}
	if roleHasAny(role, domain.PermissionLabResultViewShared) {
		return true, nil
	}
	return false, domain.ErrForbidden
}

func ownPetRecordReadOnly(role domain.Role) (bool, error) {
	if roleHasAny(role, domain.PermissionPetRecordManage) {
		return false, nil
	}
	if roleHasAny(role, domain.PermissionPetRecordManageOwn) {
		return true, nil
	}
	return false, domain.ErrForbidden
}

func ownInvoiceReadOnly(role domain.Role) (bool, error) {
	if roleHasAny(role, domain.PermissionInvoiceCreate, domain.PermissionInvoiceManage, domain.PermissionPaymentRefundVoid) {
		return false, nil
	}
	if roleHasAny(role, domain.PermissionInvoicePayOwn) {
		return true, nil
	}
	return false, domain.ErrForbidden
}

func requirePetGuardianAccess(ctx context.Context, q rowQuerier, tenantID string, petID string, actorUserID string, requireRecordView bool) error {
	if !isUUID(actorUserID) {
		return domain.ErrForbidden
	}
	var exists bool
	err := q.QueryRow(ctx, `
		select exists (
			select 1
			from pet_guardians
			where tenant_id = $1
				and pet_id = $2
				and user_id = $3
				and archived_at is null
				and ($4::boolean = false or can_view_records = true)
		)
	`, tenantID, petID, actorUserID, requireRecordView).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check pet guardian access: %w", err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return nil
}

func validManagedStaffRole(role domain.Role) bool {
	switch role {
	case domain.RoleClinicAdmin, domain.RoleVeterinarian, domain.RoleReceptionist, domain.RoleVetTechnician, domain.RoleLabTechnician:
		return true
	default:
		return false
	}
}

func validQueueStatus(status domain.QueueStatus) bool {
	switch status {
	case domain.QueueWaiting, domain.QueueCalled, domain.QueueInProgress, domain.QueueCompleted, domain.QueueCancelled:
		return true
	default:
		return false
	}
}

func validLabOrderStatus(status domain.LabOrderStatus) bool {
	switch status {
	case domain.LabOrdered, domain.LabSampleCollected, domain.LabSentOut, domain.LabInProgress, domain.LabCompleted, domain.LabCancelled:
		return true
	default:
		return false
	}
}

func labOrderTransitionAllowed(current domain.LabOrderStatus, next domain.LabOrderStatus) bool {
	if current == next {
		return true
	}
	switch current {
	case domain.LabOrdered:
		return next == domain.LabSampleCollected || next == domain.LabSentOut || next == domain.LabInProgress || next == domain.LabCancelled
	case domain.LabSampleCollected:
		return next == domain.LabSentOut || next == domain.LabInProgress || next == domain.LabCompleted || next == domain.LabCancelled
	case domain.LabSentOut:
		return next == domain.LabInProgress || next == domain.LabCompleted || next == domain.LabCancelled
	case domain.LabInProgress:
		return next == domain.LabCompleted || next == domain.LabCancelled
	default:
		return false
	}
}

func queueTransitionAllowed(current domain.QueueStatus, next domain.QueueStatus) bool {
	switch current {
	case domain.QueueWaiting:
		return next == domain.QueueCalled || next == domain.QueueInProgress || next == domain.QueueCancelled
	case domain.QueueCalled:
		return next == domain.QueueInProgress || next == domain.QueueCompleted || next == domain.QueueCancelled
	case domain.QueueInProgress:
		return next == domain.QueueCompleted || next == domain.QueueCancelled
	default:
		return false
	}
}

func queueStatusUpdateSQL(status domain.QueueStatus) (string, string) {
	switch status {
	case domain.QueueCalled:
		return `
			update queue_entries
			set status = 'called',
				called_at = now()
			where tenant_id = $1 and id = $2
		`, string(domain.AppointmentCheckedIn)
	case domain.QueueInProgress:
		return `
			update queue_entries
			set status = 'in_progress',
				called_at = coalesce(called_at, now())
			where tenant_id = $1 and id = $2
		`, string(domain.AppointmentInProgress)
	case domain.QueueCompleted:
		return `
			update queue_entries
			set status = 'completed',
				completed_at = now()
			where tenant_id = $1 and id = $2
		`, string(domain.AppointmentCompleted)
	case domain.QueueCancelled:
		return `
			update queue_entries
			set status = 'cancelled',
				cancelled_at = now()
			where tenant_id = $1 and id = $2
		`, ""
	default:
		return "", ""
	}
}

func parseOptionalTime(value *string) (any, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func mutationHash(value any) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func idempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.AppointmentMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.AppointmentMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AppointmentMutationResult{}, false, nil
	}
	if err != nil {
		return domain.AppointmentMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.AppointmentMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.AppointmentMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.AppointmentMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentQueueResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.QueueMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.QueueMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QueueMutationResult{}, false, nil
	}
	if err != nil {
		return domain.QueueMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.QueueMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.QueueMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.QueueMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentPetResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.PetMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.PetMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetMutationResult{}, false, nil
	}
	if err != nil {
		return domain.PetMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.PetMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.PetMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.PetMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentPetDocumentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.PetDocumentMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.PetDocumentMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetDocumentMutationResult{}, false, nil
	}
	if err != nil {
		return domain.PetDocumentMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.PetDocumentMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.PetDocumentMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.PetDocumentMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentPetDocumentUploadURLResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.PetDocumentUploadURLResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.PetDocumentUploadURLResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetDocumentUploadURLResult{}, false, nil
	}
	if err != nil {
		return domain.PetDocumentUploadURLResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.PetDocumentUploadURLResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.PetDocumentUploadURLResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.PetDocumentUploadURLResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentPetDocumentDownloadURLResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.PetDocumentDownloadURLResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.PetDocumentDownloadURLResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PetDocumentDownloadURLResult{}, false, nil
	}
	if err != nil {
		return domain.PetDocumentDownloadURLResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.PetDocumentDownloadURLResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.PetDocumentDownloadURLResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.PetDocumentDownloadURLResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentLabOrderResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.LabOrderMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.LabOrderMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.LabOrderMutationResult{}, false, nil
	}
	if err != nil {
		return domain.LabOrderMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.LabOrderMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.LabOrderMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.LabOrderMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentPrescriptionResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.PrescriptionMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.PrescriptionMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PrescriptionMutationResult{}, false, nil
	}
	if err != nil {
		return domain.PrescriptionMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.PrescriptionMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.PrescriptionMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.PrescriptionMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentClinicalNoteResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.ClinicalNoteMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.ClinicalNoteMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClinicalNoteMutationResult{}, false, nil
	}
	if err != nil {
		return domain.ClinicalNoteMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.ClinicalNoteMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.ClinicalNoteMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.ClinicalNoteMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentInvoiceResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.InvoiceMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.InvoiceMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.InvoiceMutationResult{}, false, nil
	}
	if err != nil {
		return domain.InvoiceMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.InvoiceMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.InvoiceMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.InvoiceMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentStaffResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.StaffMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.StaffMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.StaffMutationResult{}, false, nil
	}
	if err != nil {
		return domain.StaffMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.StaffMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.StaffMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.StaffMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentTenantResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.TenantMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.TenantMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.TenantMutationResult{}, false, nil
	}
	if err != nil {
		return domain.TenantMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.TenantMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.TenantMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.TenantMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func idempotentClinicLocationResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string) (domain.ClinicLocationMutationResult, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return domain.ClinicLocationMutationResult{}, false, nil
	}

	var existingHash string
	var body []byte
	err := tx.QueryRow(ctx, `
		select request_hash, response_body
		from idempotency_keys
		where tenant_id = $1 and key = $2 and expires_at > now()
	`, tenantID, key).Scan(&existingHash, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ClinicLocationMutationResult{}, false, nil
	}
	if err != nil {
		return domain.ClinicLocationMutationResult{}, false, fmt.Errorf("read idempotency key: %w", err)
	}
	if existingHash != requestHash {
		return domain.ClinicLocationMutationResult{}, true, fmt.Errorf("%w: idempotency key was already used with a different request", domain.ErrConflict)
	}
	var result domain.ClinicLocationMutationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.ClinicLocationMutationResult{}, false, fmt.Errorf("decode idempotent response: %w", err)
	}
	result.Idempotent = true
	return result, true, nil
}

func rememberIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.AppointmentMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberQueueIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.QueueMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberPetIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.PetMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberPetDocumentIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.PetDocumentMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberPetDocumentUploadURLResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.PetDocumentUploadURLResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberPetDocumentDownloadURLResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.PetDocumentDownloadURLResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberLabOrderIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.LabOrderMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberPrescriptionIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.PrescriptionMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberClinicalNoteIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.ClinicalNoteMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberInvoiceIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.InvoiceMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberStaffIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.StaffMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberTenantIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.TenantMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func rememberClinicLocationIdempotentResult(ctx context.Context, tx pgx.Tx, tenantID string, key string, requestHash string, status int, result domain.ClinicLocationMutationResult) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode idempotent response: %w", err)
	}
	_, err = tx.Exec(ctx, `
		insert into idempotency_keys (tenant_id, key, request_hash, response_status, response_body)
		values ($1, $2, $3, $4, $5)
		on conflict (tenant_id, key) do nothing
	`, tenantID, key, requestHash, status, body)
	if err != nil {
		return fmt.Errorf("record idempotency key: %w", err)
	}
	return nil
}

func audit(ctx context.Context, tx pgx.Tx, tenantID string, actorUserID string, actorRole domain.Role, action string, resourceType string, resourceID string, reason string) error {
	_, err := tx.Exec(ctx, `
		insert into audit_logs (
			tenant_id,
			actor_user_id,
			actor_role,
			action,
			resource_type,
			resource_id,
			reason
		)
		values ($1, $2, $3, $4, $5, $6, $7)
	`, tenantID, uuidOrNil(actorUserID), string(actorRole), action, resourceType, uuidOrNil(resourceID), nullString(reason))
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

func uuidOrNil(value string) any {
	value = strings.TrimSpace(value)
	if !isUUID(value) {
		return nil
	}
	return value
}

func nullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("create password salt: %w", err)
	}
	iterations := 210_000
	hash, err := pbkdf2.Key(sha256.New, password, salt, iterations, 32)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return fmt.Sprintf(
		"pbkdf2-sha256$%d$%s$%s",
		iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func passwordMatches(storedHash string, password string) bool {
	if storedHash == "local-dev-only" {
		return password == "local-dev-only"
	}

	parts := strings.Split(storedHash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 100_000 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 8 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) < 32 {
		return false
	}

	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return hmac.Equal(actual, expected)
}

func isUUID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 36 {
		return false
	}
	for i, ch := range value {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
				return false
			}
		}
	}
	return true
}
