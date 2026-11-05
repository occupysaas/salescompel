// Package migration owns explicit, forward-only database changes.
package migration

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"time"
)

//go:embed legacy/* migrations/* v1.go
var sources embed.FS

const LockID int64 = 739102

type Entry struct {
	Version  int64  `json:"version"`
	Name     string `json:"name"`
	Checksum string `json:"checksum"`
	Applied  bool   `json:"applied"`
}
type change struct {
	entry Entry
	run   func(context.Context, pgx.Tx) error
}

func manifest() []change {
	hash := sha256.New()
	files, _ := sources.ReadDir("legacy")
	for _, f := range files {
		v, _ := sources.ReadFile("legacy/" + f.Name())
		hash.Write([]byte(f.Name()))
		hash.Write(v)
	}
	v1, _ := sources.ReadFile("v1.go")
	hash.Write(v1)
	sql, _ := sources.ReadFile("migrations/0002_tenant_catalog.sql")
	sum := sha256.Sum256(sql)
	m := []change{{Entry{1, "legacy baseline", hex.EncodeToString(hash.Sum(nil)), false}, adopt}, {Entry{2, "tenant catalog", hex.EncodeToString(sum[:]), false}, func(ctx context.Context, tx pgx.Tx) error { _, e := tx.Exec(ctx, string(sql)); return e }}}

	sql3, _ := sources.ReadFile("migrations/0003_tenant_rls.sql")
	sum3 := sha256.Sum256(sql3)
	m = append(m, change{Entry{3, "tenant row security", hex.EncodeToString(sum3[:]), false}, func(ctx context.Context, tx pgx.Tx) error { _, e := tx.Exec(ctx, string(sql3)); return e }})
	return m

}
func history(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}) ([]Entry, error) {
	var exists bool
	if e := db.QueryRow(ctx, "SELECT to_regclass('sc_control.schema_migrations') IS NOT NULL").Scan(&exists); e != nil {
		return nil, e
	}
	if !exists {
		return nil, nil
	}
	rows, e := db.Query(ctx, "SELECT version,name,checksum FROM sc_control.schema_migrations ORDER BY version")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var r Entry
		if e = rows.Scan(&r.Version, &r.Name, &r.Checksum); e != nil {
			return nil, e
		}
		r.Applied = true
		out = append(out, r)
	}
	return out, rows.Err()
}
func checked(h []Entry) ([]change, error) {
	m := manifest()
	if len(h) > len(m) {
		return nil, errors.New("database migration version is newer than binary")
	}
	for i, r := range h {
		if r.Version != m[i].entry.Version || r.Checksum != m[i].entry.Checksum || r.Name != m[i].entry.Name {
			return nil, errors.New("database migration history gap or checksum mismatch")
		}
	}
	return m, nil
}
func Status(ctx context.Context, pool *pgxpool.Pool) ([]Entry, error) {
	h, e := history(ctx, pool)
	if e != nil {
		return nil, e
	}
	m, e := checked(h)
	if e != nil {
		return nil, e
	}
	out := []Entry{}
	for i, c := range m {
		r := c.entry
		r.Applied = i < len(h)
		out = append(out, r)
	}
	return out, nil
}
func CheckHead(ctx context.Context, pool *pgxpool.Pool) error {
	h, e := history(ctx, pool)
	if e != nil {
		return e
	}
	m, e := checked(h)
	if e != nil {
		return e
	}
	if len(h) != len(m) {
		return errors.New("database migration required: run salescompel migrate up with MIGRATION_DATABASE_URL")
	}
	return nil
}
func Up(ctx context.Context, pool *pgxpool.Pool) error { return up(ctx, pool, manifest()) }
func up(ctx context.Context, pool *pgxpool.Pool, changes []change) error {
	c, e := pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer c.Release()
	if _, e = c.Exec(ctx, "SELECT pg_advisory_lock($1)", LockID); e != nil {
		return e
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := c.Exec(cleanup, "SELECT pg_advisory_unlock($1)", LockID); err != nil {
			c.Conn().Close(cleanup)
		}
	}()
	h, e := history(ctx, c)
	if e != nil {
		return e
	}
	m, e := checked(h)
	if e != nil {
		return e
	}
	_ = m
	for _, v := range changes[len(h):] {
		tx, e := c.Begin(ctx)
		if e != nil {
			return e
		}
		e = func() error {
			defer tx.Rollback(context.Background())
			if _, e := tx.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS sc_control; CREATE TABLE IF NOT EXISTS sc_control.schema_migrations(version bigint PRIMARY KEY,name text NOT NULL,checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())"); e != nil {
				return e
			}
			if e := v.run(ctx, tx); e != nil {
				return e
			}
			if _, e := tx.Exec(ctx, "INSERT INTO sc_control.schema_migrations(version,name,checksum) VALUES($1,$2,$3)", v.entry.Version, v.entry.Name, v.entry.Checksum); e != nil {
				return e
			}
			return tx.Commit(ctx)
		}()
		if e != nil {
			return fmt.Errorf("migration %d failed: %w", v.entry.Version, e)
		}
	}
	return nil
}

// Quote permits generated identifiers only; user values are always parameters.
func Quote(parts ...string) string { return pgx.Identifier(parts).Sanitize() }
