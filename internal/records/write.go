package records

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"maps"
	"salescompel/internal/metadata"
	"salescompel/internal/recordid"
	"salescompel/internal/soql"
	"sort"
	"strings"
	"time"
)

type database interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func dbError(err error) error { return dbErrorFor(err, "") }

// dbErrorFor maps storage failures to API errors. op is "delete" for record
// deletion, where a restricting foreign key means the record is still referenced.
func dbErrorFor(err error, op string) error {
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
			if strings.Contains(pe.ConstraintName, "compat_key") {
				return fieldProblem("DUPLICATE_VALUE", "External ID already exists", "Compat_Key__c")
			}
			return invalid("DUPLICATE_VALUE", "Duplicate value")
		case "23503":
			if op == "delete" && strings.HasPrefix(pe.ConstraintName, "fk_") {
				return invalid("DELETE_FAILED", "Record is referenced by another record")
			}
			return invalid("INVALID_CROSS_REFERENCE_KEY", "Relationship is unavailable or still referenced")
		case "23502", "23514":
			return invalid("REQUIRED_FIELD_MISSING", "Required value is missing")
		case "22001":
			return invalid("STRING_TOO_LONG", "Field value too long")
		case "22003":
			return invalid("NUMBER_OUTSIDE_VALID_RANGE", "Number is outside the field's valid range")
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

// columns returns quoted columns, SQL parameters and per-parameter casts.
// Unknown fields and values that do not coerce fail the write.
func columns(p Principal, object string, data map[string]any) ([]string, []any, []string, error) {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cols, values, casts := []string{}, []any{}, []string{}
	for _, k := range keys {
		f, fErr := field(p, object, k)
		if fErr != nil {
			return nil, nil, nil, fErr
		}
		v, cast, coerceErr := coerce(f, data[k])
		if coerceErr != nil {
			return nil, nil, nil, coerceErr
		}
		if strings.EqualFold(object, "Account") && f.Name == "Compat_Key__c" && f.ExternalID {
			var canonical any
			if text, ok := v.(string); ok {
				key, cErr := canonicalExternalID(text)
				if cErr != nil {
					key = ""
				}
				if key != "" {
					canonical = key
				} else {
					v = nil
				}
			}
			cols = append(cols, pgx.Identifier{"compat_key_canonical"}.Sanitize())
			values = append(values, canonical)
			casts = append(casts, "")
		}
		cols = append(cols, pgx.Identifier{f.Column}.Sanitize())
		values = append(values, v)
		casts = append(casts, cast)
	}
	return cols, values, casts, nil
}

func create(ctx context.Context, db database, p Principal, object string, data map[string]any) (string, error) {
	o, err := writeAccess(p, object, "create", data)
	if err != nil {
		return "", err
	}
	if metadata.IsSetupObject(o.Name) {
		return setupWrite(ctx, db, p, "create", o.Name, "", data)
	}
	data = maps.Clone(data)
	if err := ownerOnCreate(ctx, db, p, o, data); err != nil {
		return "", err
	}
	return insertRecord(ctx, db, p, o.Name, data, "", nil)
}

// insertRecord writes a new row. id is generated when empty; extra holds
// physical columns that no catalog field maps (setup rows only).
func insertRecord(ctx context.Context, db database, p Principal, object string, data map[string]any, id string, extra map[string]any) (string, error) {
	var err error
	data, err = canonicalFields(p, object, data)
	if err != nil {
		return "", err
	}
	data, err = normalizeReferences(p, object, data)
	if err != nil {
		return "", err
	}
	o, err := p.Catalog.Object(object)
	if err != nil {
		return "", err
	}
	applyDefaults(o, data)
	if err := Validate(p, object, data, true); err != nil {
		return "", err
	}
	if owner, ok := data["OwnerId"].(string); ok && hasOwner(o) {
		if err := checkOwner(ctx, db, p, object, owner); err != nil {
			return "", err
		}
	}
	if err := checkLookups(ctx, db, p, object, data); err != nil {
		return "", err
	}
	if id == "" {
		if id, err = recordid.New(o.Prefix); err != nil {
			return "", dbError(err)
		}
	}
	cols, values, casts, err := columns(p, object, data)
	if err != nil {
		return "", err
	}
	numbers, err := allocateAutoNumbers(ctx, db, p, object, time.Now())
	if err != nil {
		return "", err
	}
	for _, n := range numbers {
		cols, values, casts = append(cols, pgx.Identifier{n.column}.Sanitize()), append(values, n.value), append(casts, "")
	}
	cols, values, casts = appendExtra(cols, values, casts, extra)
	cols = append([]string{"tenant_id", "id"}, cols...)
	values = append([]any{p.Tenant, id}, values...)
	casts = append([]string{"", ""}, casts...)
	marks := make([]string, len(values))
	for i := range marks {
		marks[i] = fmt.Sprintf("$%d%s", i+1, casts[i])
	}
	_, err = db.Exec(ctx, "INSERT INTO "+table(p, object)+" ("+strings.Join(cols, ",")+") VALUES ("+strings.Join(marks, ",")+")", values...)
	return id, dbError(err)
}

func appendExtra(cols []string, values []any, casts []string, extra map[string]any) ([]string, []any, []string) {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		cols, values, casts = append(cols, pgx.Identifier{k}.Sanitize()), append(values, extra[k]), append(casts, "")
	}
	return cols, values, casts
}

func update(ctx context.Context, db database, p Principal, object, id string, data map[string]any) error {
	o, err := writeAccess(p, object, "update", data)
	if err != nil {
		return err
	}
	if metadata.IsSetupObject(o.Name) {
		_, err = setupWrite(ctx, db, p, "update", o.Name, id, data)
		return err
	}
	transfer, err := checkUpdate(ctx, db, p, o, id, data)
	if err != nil {
		return err
	}
	if err = updateRecord(ctx, db, p, o.Name, id, data, nil); err != nil || transfer == nil {
		return err
	}
	return transfer()
}

// updateRecord changes one row; extra is as for insertRecord.
func updateRecord(ctx context.Context, db database, p Principal, object, id string, data map[string]any, extra map[string]any) error {
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
	if owner, ok := data["OwnerId"].(string); ok {
		if err := checkOwner(ctx, db, p, object, owner); err != nil {
			return err
		}
	}
	if err := checkLookups(ctx, db, p, object, data); err != nil {
		return err
	}
	cols, values, casts, err := columns(p, object, data)
	if err != nil {
		return err
	}
	cols, values, casts = appendExtra(cols, values, casts, extra)
	sets := []string{}
	for i, c := range cols {
		sets = append(sets, fmt.Sprintf("%s=$%d%s", c, i+3, casts[i]))
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
	o, err := writeAccess(p, object, "delete", nil)
	if err != nil {
		return err
	}
	if metadata.IsSetupObject(o.Name) {
		_, err = setupWrite(ctx, db, p, "delete", o.Name, id, nil)
		return err
	}
	nid, err := checkDelete(ctx, db, p, o, id)
	if err != nil {
		return err
	}
	if err = removeRecord(ctx, db, p, o.Name, id); err != nil {
		return err
	}
	return dropShares(ctx, db, p, nid)
}
func removeRecord(ctx context.Context, db database, p Principal, object, id string) error {
	id, err := normalizeID(p, object, id)
	if err != nil {
		return dbError(pgx.ErrNoRows)
	}
	tag, err := db.Exec(ctx, "DELETE FROM "+table(p, object)+" WHERE tenant_id=$1 AND id=$2", p.Tenant, id)
	if err != nil {
		return dbErrorFor(err, "delete")
	}
	if tag.RowsAffected() == 0 {
		return dbError(pgx.ErrNoRows)
	}
	return nil
}

// Read returns one record. fields, from REST ?fields=, limits the columns;
// a hidden or unknown name is INVALID_FIELD (D33-03). Without fields every
// readable field is returned.
func (s *Tx) Read(ctx context.Context, object, id string, fields ...string) (map[string]any, error) {
	p := s.principal
	d, err := Describe(p, object)
	if err != nil {
		return nil, err
	}
	names, err := readColumns(p, d, fields)
	if err != nil {
		return nil, err
	}
	id, err = normalizeID(p, object, id)
	if err != nil {
		return nil, dbError(pgx.ErrNoRows)
	}
	text := "SELECT " + strings.Join(names, ",") + " FROM " + object + " WHERE Id = '" + id + "' LIMIT 1"
	res, err := s.Query(ctx, text, soql.Options{BatchSize: 1})
	if err != nil {
		return nil, err
	}
	if res.TotalSize == 0 {
		return nil, dbError(pgx.ErrNoRows)
	}
	return res.Records[0], nil
}

// SelectFields resolves the fields a Collections retrieve names, through the
// same permission-aware lookup as Read (D33-03), and adds Id, which retrieve
// uses to put each row back at its requested position.
func SelectFields(p Principal, object string, fields []string) ([]string, error) {
	d, err := Describe(p, object)
	if err != nil {
		return nil, err
	}
	names, err := readColumns(p, d, fields)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		if n == "Id" {
			return names, nil
		}
	}
	return append(names, "Id"), nil
}
func (s *Tx) UpsertAccount(ctx context.Context, key string, data map[string]any) (string, bool, error) {
	p := s.principal
	if _, err := writeAccess(p, "Account", "upsert", data); err != nil {
		return "", false, err
	}
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
	maps.Copy(copyData, data)
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
	if err == nil {
		// A match the caller can't read is a duplicate, never an update
		// (D34-29).
		account, _ := p.Catalog.Object("Account")
		if read, _, aerr := recordAccess(ctx, tx, p, account, id, soql.LevelRead); aerr != nil {
			return "", false, aerr
		} else if !read {
			return "", false, fieldProblem("DUPLICATE_VALUE", "External ID already exists", "Compat_Key__c")
		}
	}
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

// errObjectNotFound is REST's answer for an unknown object, also given for
// a hidden one (D33-02) so the two can't be told apart on any write path.
var errObjectNotFound = problem(404, "NOT_FOUND", "Object not found")

// errInsufficientAccess is the 403 for a missing object permission (D33-06),
// Manage Users (D33-08), a profile-owned row (D33-21) or an operation the
// object doesn't support (D33-31).
var errInsufficientAccess = problem(403, "INSUFFICIENT_ACCESS_OR_READONLY", "insufficient access rights on object id")

// writeAccess checks a write against the object and its operation flags,
// then, outside the setup objects (whose rules are in setupWrite), the
// object permission for op and the field permissions for data. op is
// create, update, delete or upsert (create or edit).
func writeAccess(p Principal, object, op string, data map[string]any) (Object, error) {
	o, err := p.Catalog.Object(object)
	if err != nil || !Readable(p, o.Name) {
		return Object{}, errObjectNotFound // D33-02
	}
	if !(op == "create" && o.Createable || op == "update" && o.Updateable || op == "delete" && o.Deletable || op == "upsert" && (o.Createable || o.Updateable)) {
		return Object{}, errInsufficientAccess // D33-31
	}
	if metadata.IsSetupObject(o.Name) {
		return o, nil
	}
	a := p.Perms.Object(o.Name)
	if !(op == "create" && a.Create || op == "update" && a.Edit || op == "delete" && a.Delete || op == "upsert" && (a.Create || a.Edit)) {
		return Object{}, errInsufficientAccess // D33-06
	}
	return o, fieldWriteAccess(p, o, data)
}

// fieldWriteAccess refuses a field the caller can't read as nonexistent
// (D33-04) and a readable one they can't edit (D33-05). Names the catalog
// doesn't know, and fields no one can write, are left to Validate.
func fieldWriteAccess(p Principal, o Object, data map[string]any) error {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, f := range o.Fields {
			if !strings.EqualFold(f.Name, k) {
				continue
			}
			if !p.Perms.FieldRead(o.Name, f) {
				return fieldProblem("INVALID_FIELD", "No such column '"+k+"' on sobject of type "+o.Name, k) // D33-04
			}
			if (f.Createable || f.Updateable) && !p.Perms.FieldEdit(o.Name, f) {
				return fieldProblem("INVALID_FIELD_FOR_INSERT_UPDATE", "Unable to create/update fields: "+f.Name+
					". Please check the security settings of this field and verify that it is read-only for this user.", f.Name) // D33-05
			}
		}
	}
	return nil
}
