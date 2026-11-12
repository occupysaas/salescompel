package records

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"salescompel/internal/query"
	"salescompel/internal/recordid"
	"sort"
	"strings"
)

type Query = query.Query
type database interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
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
			return fieldProblem("DUPLICATE_VALUE", "External ID already exists", "Compat_Key__c")
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
func table(p Principal, object string) string {
	if p.Catalog != nil {
		o, e := p.Catalog.Object(object)
		if e == nil {
			return o.TableSQL()
		}
		return "invalid_object"
	}
	if object == "Account" {
		return "accounts"
	}
	return "invalid_object"
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
		if strings.EqualFold(object, "Account") && f.Name == "Compat_Key__c" && f.ExternalID {
			var canonical any
			if text, ok := v.(string); ok {
				key, _ := canonicalExternalID(text)
				if key != "" {
					canonical = key
				} else {
					v = nil
				}
			}
			cols = append(cols, pgx.Identifier{"compat_key_canonical"}.Sanitize())
			values = append(values, canonical)
		}
		cols = append(cols, pgx.Identifier{f.Column}.Sanitize())
		values = append(values, v)
	}
	return cols, values
}
func create(ctx context.Context, db database, p Principal, object string, data map[string]any) (string, error) {
	var err error
	data, err = canonicalFields(p, object, data)
	if err != nil {
		return "", err
	}
	data, err = normalizeReferences(p, object, data)
	if err != nil {
		return "", err
	}
	if err := Validate(p, object, data, true); err != nil {
		return "", err
	}
	prefix := "001"
	if object == "Contact" {
		prefix = "003"
	}
	if p.Catalog != nil {
		o, e := p.Catalog.Object(object)
		if e != nil {
			return "", e
		}
		prefix = o.Prefix
	}
	id, err := recordid.New(prefix)
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
	_, err = db.Exec(ctx, "INSERT INTO "+table(p, object)+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", values...)
	return id, dbError(err)
}
func update(ctx context.Context, db database, p Principal, object, id string, data map[string]any) error {
	var err error
	id, err = normalizeID(p, object, id)
	if err != nil {
		return dbError(pgx.ErrNoRows)
	}
	data, err = canonicalFields(p, object, data)
	if err != nil {
		return err
	}
	data, err = normalizeReferences(p, object, data)
	if err != nil {
		return err
	}
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
	tag, err := db.Exec(ctx, "UPDATE "+table(p, object)+" SET "+strings.Join(sets, ",")+" WHERE tenant_id=$1 AND id=$2", values...)
	if err != nil {
		return dbError(err)
	}
	if tag.RowsAffected() == 0 {
		return dbError(pgx.ErrNoRows)
	}
	return nil
}
func deleteRecord(ctx context.Context, db database, p Principal, object, id string) error {
	if _, err := Describe(p, object); err != nil {
		return err
	}
	id, err := normalizeID(p, object, id)
	if err != nil {
		return dbError(pgx.ErrNoRows)
	}
	tag, err := db.Exec(ctx, "DELETE FROM "+table(p, object)+" WHERE tenant_id=$1 AND id=$2", p.Tenant, id)
	if err != nil {
		return dbError(err)
	}
	if tag.RowsAffected() == 0 {
		return dbError(pgx.ErrNoRows)
	}
	return nil
}
func (s *Tx) Read(ctx context.Context, object, id string) (map[string]any, error) {
	p := s.principal
	d, err := Describe(p, object)
	if err != nil {
		return nil, err
	}
	id, err = normalizeID(p, object, id)
	if err != nil {
		return nil, dbError(pgx.ErrNoRows)
	}
	q := Query{Object: object, HasWhere: true, WhereField: "Id", WhereValue: &id, Limit: 1}
	for _, f := range d.Fields {
		if !f.Compound() {
			q.Fields = append(q.Fields, f.Name)
		}
	}
	rows, err := s.Select(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, dbError(pgx.ErrNoRows)
	}
	return rows[0], nil
}
func (s *Tx) UpsertAccount(ctx context.Context, key string, data map[string]any) (string, bool, error) {
	p := s.principal
	var err error
	data, err = canonicalFields(p, "Account", data)
	if err != nil {
		return "", false, err
	}
	if supplied := data["Compat_Key__c"]; supplied != nil {
		switch supplied.(type) {
		case map[string]any, []any:
			return "", false, invalid("JSON_PARSER_ERROR", "External ID requires a scalar value or null")
		default:
			return "", false, invalid("INVALID_FIELD", "External ID value must be supplied in the upsert URL")
		}
	}
	copyData := map[string]any{}
	for k, v := range data {
		copyData[k] = v
	}
	copyData["Compat_Key__c"] = key
	if err := Validate(p, "Account", copyData, false); err != nil {
		return "", false, err
	}
	canonical, err := canonicalExternalID(key)
	if err != nil || canonical == "" {
		return "", false, fieldProblem("INVALID_FIELD", "External ID must not be blank", "Compat_Key__c")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", false, dbError(err)
	}
	defer tx.Rollback(ctx)
	// Serialize the read/create decision for this tenant/key, across all server processes.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", p.Tenant+"\x1f"+canonical); err != nil {
		return "", false, dbError(err)
	}
	var id string
	err = tx.QueryRow(ctx, "SELECT id FROM accounts WHERE tenant_id=$1 AND compat_key_canonical=$2 FOR UPDATE", p.Tenant, canonical).Scan(&id)
	created := errors.Is(err, pgx.ErrNoRows)
	if created {
		id, err = create(ctx, tx, p, "Account", copyData)
	} else if err == nil {
		delete(copyData, "Compat_Key__c")
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
