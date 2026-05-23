package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const advisoryLockKey int64 = 73911001

type Runner struct {
	pool *pgxpool.Pool
}

type AppliedMigration struct {
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
}

type MigrationStatus struct {
	Name      string     `json:"name"`
	Checksum  string     `json:"checksum"`
	Applied   bool       `json:"applied"`
	AppliedAt *time.Time `json:"appliedAt,omitempty"`
}

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database URL is required")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func NewRunner(pool *pgxpool.Pool) Runner {
	return Runner{pool: pool}
}

func (r Runner) Up(ctx context.Context) ([]AppliedMigration, error) {
	return r.apply(ctx, "up")
}

func (r Runner) Down(ctx context.Context) ([]AppliedMigration, error) {
	return r.apply(ctx, "down")
}

func (r Runner) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := r.ensureMetadata(ctx); err != nil {
		return nil, err
	}

	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}
	up, err := Migrations("up")
	if err != nil {
		return nil, err
	}

	status := make([]MigrationStatus, 0, len(up))
	for _, migration := range up {
		version := migration.Version()
		item := MigrationStatus{
			Name:     migration.Name,
			Checksum: migration.Checksum(),
		}
		if row, ok := applied[version]; ok {
			item.Applied = true
			item.AppliedAt = &row.AppliedAt
		}
		status = append(status, item)
	}

	return status, nil
}

func (r Runner) apply(ctx context.Context, direction string) ([]AppliedMigration, error) {
	if err := r.ensureMetadata(ctx); err != nil {
		return nil, err
	}

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "select pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return nil, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "select pg_advisory_unlock($1)", advisoryLockKey)
	}()

	applied, err := r.applied(ctx)
	if err != nil {
		return nil, err
	}
	migrations, err := Migrations(direction)
	if err != nil {
		return nil, err
	}

	changed := make([]AppliedMigration, 0, len(migrations))
	for _, migration := range migrations {
		version := migration.Version()
		checksum := migration.Checksum()
		_, alreadyApplied := applied[version]

		if direction == "up" && alreadyApplied {
			continue
		}
		if direction == "down" && !alreadyApplied {
			continue
		}

		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return nil, fmt.Errorf("begin migration %s: %w", migration.Name, err)
		}

		if err := executeStatements(ctx, tx, migration.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("execute migration %s: %w", migration.Name, err)
		}

		if direction == "up" {
			if _, err := tx.Exec(ctx, "insert into schema_migrations (name, checksum) values ($1, $2)", version, checksum); err != nil {
				_ = tx.Rollback(ctx)
				return nil, fmt.Errorf("record migration %s: %w", migration.Name, err)
			}
		} else {
			if _, err := tx.Exec(ctx, "delete from schema_migrations where name = $1", version); err != nil {
				_ = tx.Rollback(ctx)
				return nil, fmt.Errorf("forget migration %s: %w", migration.Name, err)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit migration %s: %w", migration.Name, err)
		}

		changed = append(changed, AppliedMigration{Name: migration.Name, Checksum: checksum})
	}

	return changed, nil
}

func (r Runner) ensureMetadata(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		create table if not exists schema_migrations (
			name text primary key,
			checksum text not null,
			applied_at timestamptz not null default now()
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

type appliedMigration struct {
	Checksum  string
	AppliedAt time.Time
}

func (r Runner) applied(ctx context.Context) (map[string]appliedMigration, error) {
	rows, err := r.pool.Query(ctx, "select name, checksum, applied_at from schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema migrations: %w", err)
	}
	defer rows.Close()

	items := map[string]appliedMigration{}
	for rows.Next() {
		var name string
		var item appliedMigration
		if err := rows.Scan(&name, &item.Checksum, &item.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan schema migration: %w", err)
		}
		items[name] = item
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migrations: %w", err)
	}
	return items, nil
}

func (m Migration) Version() string {
	name := strings.TrimSuffix(m.Name, ".up.sql")
	name = strings.TrimSuffix(name, ".down.sql")
	return name
}

func (m Migration) Checksum() string {
	sum := sha256.Sum256([]byte(m.SQL))
	return hex.EncodeToString(sum[:])
}

func executeStatements(ctx context.Context, tx pgx.Tx, sql string) error {
	for _, statement := range splitSQLStatements(sql) {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func splitSQLStatements(sql string) []string {
	statements := []string{}
	var current strings.Builder
	inSingleQuote := false
	runes := []rune(sql)

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		current.WriteRune(ch)

		if ch == '\'' {
			if inSingleQuote && i+1 < len(runes) && runes[i+1] == '\'' {
				i++
				current.WriteRune(runes[i])
				continue
			}
			inSingleQuote = !inSingleQuote
			continue
		}

		if ch == ';' && !inSingleQuote {
			statement := strings.TrimFunc(current.String(), unicode.IsSpace)
			statement = strings.TrimSuffix(statement, ";")
			statement = strings.TrimFunc(statement, unicode.IsSpace)
			if statement != "" {
				statements = append(statements, statement)
			}
			current.Reset()
		}
	}

	statement := strings.TrimFunc(current.String(), unicode.IsSpace)
	if statement != "" {
		statements = append(statements, statement)
	}

	return statements
}
