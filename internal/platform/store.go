package platform

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

type Store struct{ pool *pgxpool.Pool }
type database interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func Open(ctx context.Context, url string) (*Store, error) {
	if url == "" {
		return nil, errors.New("DATABASE_URL required")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	ok := false
	defer func() {
		if !ok {
			pool.Close()
		}
	}()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, errors.New("database unavailable")
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(739102)"); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return nil, err
	}
	if err = ensureCatalog(ctx, tx); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	ok = true
	return &Store{pool: pool}, nil
}
func (s *Store) Close()          { s.pool.Close() }
func digest(token string) string { v := sha256.Sum256([]byte(token)); return hex.EncodeToString(v[:]) }
func (s *Store) Provision(ctx context.Context, token string, p Principal) error {
	if len(token) < 16 || p.Tenant == "" || p.User == "" || (p.Role != "admin" && p.Role != "restricted") {
		return errors.New("token must be at least 16 characters; tenant, user and valid role required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "INSERT INTO tenants(id) VALUES($1) ON CONFLICT DO NOTHING", p.Tenant); err != nil {
		return err
	}
	// A token cannot silently be reassigned to a different principal.
	_, err = tx.Exec(ctx, "INSERT INTO principals(token_hash,tenant_id,user_id,role) VALUES($1,$2,$3,$4) ON CONFLICT(token_hash) DO NOTHING", digest(token), p.Tenant, p.User, p.Role)
	if err != nil {
		return err
	}
	var actual Principal
	err = tx.QueryRow(ctx, "SELECT tenant_id,user_id,role FROM principals WHERE token_hash=$1", digest(token)).Scan(&actual.Tenant, &actual.User, &actual.Role)
	if err != nil {
		return err
	}
	if actual != p {
		return errors.New("token already assigned")
	}
	return tx.Commit(ctx)
}
func (s *Store) Authenticate(ctx context.Context, token string) (Principal, error) {
	var p Principal
	if token == "" {
		return p, problem(401, "INVALID_SESSION_ID", "Bearer token required")
	}
	err := s.pool.QueryRow(ctx, "SELECT tenant_id,user_id,role FROM principals WHERE token_hash=$1", digest(token)).Scan(&p.Tenant, &p.User, &p.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, problem(401, "INVALID_SESSION_ID", "Invalid session")
	}
	if err != nil {
		return p, dbError(err)
	}
	return p, nil
}
func dbError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "NOT_FOUND", "Record not found")
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505":
			return invalid("DUPLICATE_VALUE", "External ID already exists")
		case "23503":
			return invalid("INVALID_CROSS_REFERENCE_KEY", "Relationship is unavailable or still referenced")
		case "23502", "23514":
			return invalid("REQUIRED_FIELD_MISSING", "Required value is missing")
		case "22001":
			return invalid("STRING_TOO_LONG", "Field value too long")
		}
	}
	return problem(500, "SERVER_UNAVAILABLE", "Storage operation failed")
}
func table(object string) string {
	if object == "Account" {
		return "accounts"
	}
	return "contacts"
}
func newID(object string) (string, error) {
	var b [9]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	prefix := "001"
	if object == "Contact" {
		prefix = "003"
	}
	var out strings.Builder
	out.WriteString(prefix)
	// 72 random bits, represented by twelve base64-like alphanumeric characters.
	var acc uint64
	for _, x := range b[:8] {
		acc = acc<<8 | uint64(x)
	}
	for i := 0; i < 11; i++ {
		out.WriteByte(alphabet[acc%62])
		acc /= 62
	}
	out.WriteByte(alphabet[int(b[8])%62])
	base := out.String()
	checksum := "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	for group := 0; group < 3; group++ {
		bits := 0
		for j := 0; j < 5; j++ {
			c := base[group*5+j]
			if c >= 'A' && c <= 'Z' {
				bits |= 1 << j
			}
		}
		out.WriteByte(checksum[bits])
	}
	return out.String(), nil
}
func columns(p Principal, object string, data map[string]any) ([]string, []any) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cols := []string{}
	values := []any{}
	for _, k := range keys {
		f, _ := field(p, object, k)
		v := data[k]
		// Salesforce does not distinguish a blank optional text value from null.
		if v == "" && f.Nillable {
			v = nil
		}
		cols = append(cols, f.Column)
		values = append(values, v)
	}
	return cols, values
}
func create(ctx context.Context, db database, p Principal, object string, data map[string]any) (string, error) {
	if err := Validate(p, object, data, true); err != nil {
		return "", err
	}
	id, err := newID(object)
	if err != nil {
		return "", dbError(err)
	}
	cols, values := columns(p, object, data)
	cols = append([]string{"tenant_id", "id"}, cols...)
	values = append([]any{p.Tenant, id}, values...)
	marks := make([]string, len(values))
	for i := range marks {
		marks[i] = fmt.Sprintf("$%d", i+1)
	}
	_, err = db.Exec(ctx, "INSERT INTO "+table(object)+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", values...)
	return id, dbError(err)
}
func (s *Store) Create(ctx context.Context, p Principal, object string, data map[string]any) (string, error) {
	return create(ctx, s.pool, p, object, data)
}
func update(ctx context.Context, db database, p Principal, object, id string, data map[string]any) error {
	if err := Validate(p, object, data, false); err != nil {
		return err
	}
	cols, values := columns(p, object, data)
	sets := []string{}
	for i, c := range cols {
		sets = append(sets, fmt.Sprintf("%s=$%d", c, i+3))
	}
	if len(sets) == 0 {
		sets = append(sets, "id=id")
	}
	values = append([]any{p.Tenant, id}, values...)
	tag, err := db.Exec(ctx, "UPDATE "+table(object)+" SET "+strings.Join(sets, ",")+" WHERE tenant_id=$1 AND id=$2", values...)
	if err != nil {
		return dbError(err)
	}
	if tag.RowsAffected() == 0 {
		return dbError(pgx.ErrNoRows)
	}
	return nil
}
func (s *Store) Update(ctx context.Context, p Principal, object, id string, data map[string]any) error {
	return update(ctx, s.pool, p, object, id, data)
}
func (s *Store) Delete(ctx context.Context, p Principal, object, id string) error {
	if _, err := Describe(p, object); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, "DELETE FROM "+table(object)+" WHERE tenant_id=$1 AND id=$2", p.Tenant, id)
	if err != nil {
		return dbError(err)
	}
	if tag.RowsAffected() == 0 {
		return dbError(pgx.ErrNoRows)
	}
	return nil
}
func (s *Store) Read(ctx context.Context, p Principal, object, id string) (map[string]any, error) {
	d, err := Describe(p, object)
	if err != nil {
		return nil, err
	}
	q := Query{Object: object, HasWhere: true, WhereField: "Id", WhereValue: &id, Limit: 1}
	for _, f := range d.Fields {
		q.Fields = append(q.Fields, f.Name)
	}
	rows, err := s.Select(ctx, p, q)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, dbError(pgx.ErrNoRows)
	}
	return rows[0], nil
}
func (s *Store) Upsert(ctx context.Context, p Principal, key string, data map[string]any) (string, bool, error) {
	copyData := map[string]any{}
	for k, v := range data {
		copyData[k] = v
	}
	if supplied, ok := copyData["Compat_Key__c"]; ok && supplied != key {
		return "", false, invalid("INVALID_FIELD", "Conflicting external ID")
	}
	copyData["Compat_Key__c"] = key
	if err := Validate(p, "Account", copyData, false); err != nil {
		return "", false, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", false, dbError(err)
	}
	defer tx.Rollback(ctx)
	// Serialize the read/create decision for this tenant/key, across all server processes.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", p.Tenant+"\x1f"+key); err != nil {
		return "", false, dbError(err)
	}
	var id string
	err = tx.QueryRow(ctx, "SELECT id FROM accounts WHERE tenant_id=$1 AND compat_key=$2 FOR UPDATE", p.Tenant, key).Scan(&id)
	created := errors.Is(err, pgx.ErrNoRows)
	if created {
		id, err = create(ctx, tx, p, "Account", copyData)
	} else if err == nil {
		err = update(ctx, tx, p, "Account", id, copyData)
	}
	if err != nil {
		return "", false, dbErrorPreserve(err)
	}
	return id, created, dbError(tx.Commit(ctx))
}
func dbErrorPreserve(err error) error {
	var api *APIError
	if errors.As(err, &api) {
		return err
	}
	return dbError(err)
}
