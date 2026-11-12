package metadata

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"salescompel/internal/fieldtype"
	"strings"
	"time"
	"unicode/utf8"
)

// ObjectInput requests a custom object. NameType is Text (default) or
// AutoNumber, which requires DisplayFormat.
type ObjectInput struct {
	Name, Label, PluralLabel string
	NameType, DisplayFormat  string
	StartingNumber           int64
}

var apiName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}__c$`)
var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func ValidAPIName(name string) bool {
	stem := strings.TrimSuffix(name, "__c")
	return apiName.MatchString(name) && !strings.Contains(stem, "__") && !strings.HasSuffix(stem, "_")
}
func validLabel(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= 80
}
func randomID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func ownerTx(ctx context.Context, p *pgxpool.Pool, tenant string, fn func(pgx.Tx) error) error {
	tx, e := p.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, "SET LOCAL ROLE salescompel_owner"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "SET LOCAL lock_timeout='5s'; SELECT pg_advisory_xact_lock(739102)"); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// lockRevision verifies the tenant exists and serializes catalog changes.
func lockRevision(ctx context.Context, tx pgx.Tx, tenant string) error {
	var revision int64
	if e := tx.QueryRow(ctx, "SELECT revision FROM sc_catalog.revisions WHERE tenant_id=$1 FOR UPDATE", tenant).Scan(&revision); e != nil {
		return errors.New("tenant is not provisioned")
	}
	return nil
}
func bumpRevision(ctx context.Context, tx pgx.Tx, tenant string) error {
	_, e := tx.Exec(ctx, "UPDATE sc_catalog.revisions SET revision=revision+1 WHERE tenant_id=$1", tenant)
	return e
}

// startCounter creates an auto-number counter and numbers existing records
// in ascending Id order, then makes the column required and unique.
func startCounter(ctx context.Context, tx pgx.Tx, tenant string, o Object, f Field) error {
	col := pgx.Identifier{f.Column}.Sanitize()
	rows, e := tx.Query(ctx, "SELECT id FROM "+o.TableSQL()+" WHERE tenant_id=$1 ORDER BY id", tenant)
	if e != nil {
		return e
	}
	ids, e := pgx.CollectRows(rows, pgx.RowTo[string])
	if e != nil {
		return e
	}
	next, now := f.Options.StartingNumber, time.Now().UTC()
	for _, id := range ids {
		v, e := fieldtype.FormatAutoNumber(f.Options.DisplayFormat, next, now)
		if e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, "UPDATE "+o.TableSQL()+" SET "+col+"=$3 WHERE tenant_id=$1 AND id=$2", tenant, id, v); e != nil {
			return e
		}
		next++
	}
	if _, e = tx.Exec(ctx, "INSERT INTO sc_catalog.auto_numbers(tenant_id,field_id,next_value) VALUES($1,$2,$3)", tenant, f.ID, next); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "ALTER TABLE "+o.TableSQL()+" ALTER COLUMN "+col+" SET NOT NULL"); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "CREATE UNIQUE INDEX "+pgx.Identifier{"uq_" + f.ID}.Sanitize()+" ON "+o.TableSQL()+"(tenant_id,"+col+")")
	return e
}

func CreateObject(ctx context.Context, p *pgxpool.Pool, tenant string, in ObjectInput) (Object, error) {
	o := Object{}
	if !ValidAPIName(in.Name) || !validLabel(in.Label) || !validLabel(in.PluralLabel) {
		return o, errors.New("invalid object name or label")
	}
	name := Field{Name: "Name", Label: "Name", Column: "name", Length: 80, Createable: true, Updateable: true, NameField: true, Options: Options{Kind: fieldtype.Text}}
	nameColumn := "name varchar(80) NOT NULL CHECK(length(trim(name))>0)"
	switch in.NameType {
	case "", "Text":
		if in.DisplayFormat != "" || in.StartingNumber != 0 {
			return o, errors.New("display format applies only to an AutoNumber name")
		}
	case "AutoNumber":
		if !fieldtype.ValidAutoNumberFormat(in.DisplayFormat) || in.StartingNumber < 0 {
			return o, errors.New("invalid auto-number display format or starting number")
		}
		name = Field{ID: randomID(), Name: "Name", Label: "Name", Column: "name", NameField: true, Options: Options{Kind: fieldtype.AutoNumber, DisplayFormat: in.DisplayFormat, StartingNumber: in.StartingNumber}}
		nameColumn = "name varchar(30)"
	default:
		return o, errors.New("name type must be Text or AutoNumber")
	}
	name = Complete(name)
	e := ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		if e := lockRevision(ctx, tx, tenant); e != nil {
			return e
		}
		var n int
		if e := tx.QueryRow(ctx, "SELECT count(*) FROM sc_catalog.objects WHERE tenant_id=$1 AND NOT standard", tenant).Scan(&n); e != nil {
			return e
		}
		if n >= 3844 {
			return errors.New("custom object prefixes exhausted")
		}
		const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
		id := randomID()
		o = Object{ID: id, Tenant: tenant, Name: in.Name, Label: in.Label, PluralLabel: in.PluralLabel, Schema: "sc_records", Table: "o_" + id, Prefix: "a" + string(alphabet[n/62]) + string(alphabet[n%62]), Createable: true, Updateable: true, Deletable: true, Queryable: true, Fields: []Field{Complete(Field{Name: "Id", Column: "id", Options: Options{Kind: fieldtype.Id}}), name}}
		if _, e := tx.Exec(ctx, "CREATE TABLE "+o.TableSQL()+"(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,"+nameColumn+",PRIMARY KEY(tenant_id,id))"); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES($1,$2,$3,$4,$5,false,$6,$7,$8)`, id, tenant, in.Name, in.Label, in.PluralLabel, o.Schema, o.Table, o.Prefix); e != nil {
			return e
		}
		for i, f := range o.Fields {
			if f.ID == "" {
				f.ID = randomID()
				o.Fields[i].ID = f.ID
			}
			if e := insertField(ctx, tx, tenant, id, f, i); e != nil {
				return e
			}
		}
		if name.Options.Kind == fieldtype.AutoNumber {
			if e := startCounter(ctx, tx, tenant, o, name); e != nil {
				return e
			}
		}
		if e := ProtectTable(ctx, tx, o.Schema, o.Table, false); e != nil {
			return e
		}
		return bumpRevision(ctx, tx, tenant)
	})
	return o, e
}

// storedNameTaken reports whether a stored-not-enforced definition reserves name.
func storedNameTaken(ctx context.Context, tx pgx.Tx, tenant, objectID, name string) (bool, error) {
	var taken bool
	e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM sc_catalog.artifacts WHERE tenant_id=$1 AND object_id=$2 AND lower(api_name)=lower($3))", tenant, objectID, name).Scan(&taken)
	return taken, e
}

func CreateField(ctx context.Context, p *pgxpool.Pool, tenant string, in FieldInput) (Field, error) {
	f := Field{}
	if _, err := fieldtype.ParseKind(in.Type); err != nil || !ValidAPIName(in.Name) || !validLabel(in.Label) {
		return f, errors.New("invalid field name, label or type")
	}
	e := ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		if e := lockRevision(ctx, tx, tenant); e != nil {
			return e
		}
		snapshot, e := Load(ctx, tx, tenant)
		if e != nil {
			return e
		}
		o, e := snapshot.Object(in.Object)
		if e != nil {
			return e
		}
		if taken, e := storedNameTaken(ctx, tx, tenant, o.ID, in.Name); e != nil || taken {
			if e == nil {
				e = errors.New("field already exists as a stored definition")
			}
			return e
		}
		var hasRecords bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM "+o.TableSQL()+" WHERE tenant_id=$1)", tenant).Scan(&hasRecords); e != nil {
			return e
		}
		var ddl ColumnDDL
		if f, ddl, e = BuildField(in, o, snapshot, hasRecords); e != nil {
			return e
		}
		col := pgx.Identifier{f.Column}.Sanitize()
		sql := "ALTER TABLE " + o.TableSQL() + " ADD COLUMN " + col + " " + ddl.Type
		if ddl.Check != "" {
			sql += " CHECK(" + ddl.Check + ")"
		}
		if ddl.ForeignKey != "" {
			sql += ", ADD " + ddl.ForeignKey
		}
		if _, e = tx.Exec(ctx, sql); e != nil {
			return e
		}
		if ddl.ForeignKey != "" {
			if _, e = tx.Exec(ctx, "CREATE INDEX "+pgx.Identifier{"ix_" + f.ID}.Sanitize()+" ON "+o.TableSQL()+"(tenant_id,"+col+")"); e != nil {
				return e
			}
		}
		if e = insertField(ctx, tx, tenant, o.ID, f, len(o.Fields)); e != nil {
			return e
		}
		if f.Options.Kind == fieldtype.AutoNumber {
			if e = startCounter(ctx, tx, tenant, o, f); e != nil {
				return e
			}
		}
		return bumpRevision(ctx, tx, tenant)
	})
	return f, e
}

// CreateStoredField retains a stored-not-enforced definition (formula,
// roll-up summary, encrypted text, geolocation or rich text). It creates no
// column and is never described; its exact source bytes are kept.
func CreateStoredField(ctx context.Context, p *pgxpool.Pool, tenant, object string, definition []byte) (StoredField, error) {
	var head struct {
		Name, Label, Type string
	}
	if e := json.Unmarshal(definition, &head); e != nil || !bytes.HasPrefix(bytes.TrimSpace(definition), []byte("{")) {
		return StoredField{}, errors.New("definition must be a JSON object")
	}
	k, err := fieldtype.ParseKind(head.Type)
	if err != nil || !k.Stored() {
		return StoredField{}, fmt.Errorf("type %q is not a stored-not-enforced type", head.Type)
	}
	if !ValidAPIName(head.Name) || !validLabel(head.Label) {
		return StoredField{}, errors.New("invalid stored definition name or label")
	}
	out := StoredField{Name: head.Name, Label: head.Label, Kind: head.Type}
	e := ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		if e := lockRevision(ctx, tx, tenant); e != nil {
			return e
		}
		snapshot, e := Load(ctx, tx, tenant)
		if e != nil {
			return e
		}
		o, e := snapshot.Object(object)
		if e != nil {
			return e
		}
		out.Object = o.Name
		for _, f := range o.Fields {
			if strings.EqualFold(f.Name, head.Name) {
				return errors.New("field already exists")
			}
		}
		if taken, e := storedNameTaken(ctx, tx, tenant, o.ID, head.Name); e != nil || taken {
			if e == nil {
				e = errors.New("field already exists as a stored definition")
			}
			return e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO sc_catalog.artifacts(id,tenant_id,source,active,object_id,api_name,kind) VALUES($1,$2,$3,false,$4,$5,$6)", randomID(), tenant, definition, o.ID, head.Name, head.Type); e != nil {
			return e
		}
		return bumpRevision(ctx, tx, tenant)
	})
	return out, e
}

// SetCurrency changes the tenant corporate currency used for UI formatting.
func SetCurrency(ctx context.Context, p *pgxpool.Pool, tenant, code string) error {
	if !currencyCode.MatchString(code) {
		return errors.New("currency must be a three-letter uppercase ISO 4217 code")
	}
	return ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, "UPDATE sc_catalog.settings SET currency_iso_code=$2 WHERE tenant_id=$1", tenant, code)
		if e == nil && tag.RowsAffected() != 1 {
			e = errors.New("tenant is not provisioned")
		}
		return e
	})
}

// Inspect renders a snapshot for the operator with each definition's status.
func Inspect(s Snapshot) map[string]any {
	objects := []map[string]any{}
	for _, o := range s.Objects {
		fields := []map[string]any{}
		for _, f := range o.Fields {
			// Best-effort rendering for the operator: a field that cannot be
			// round-tripped still appears, with its identifying names.
			raw, mErr := json.Marshal(f)
			m := map[string]any{}
			if mErr != nil || json.Unmarshal(raw, &m) != nil {
				m = map[string]any{"name": f.Name, "label": f.Label}
			}
			m["status"] = "active"
			fields = append(fields, m)
		}
		for _, sf := range s.Stored {
			if sf.Object == o.Name {
				fields = append(fields, map[string]any{"name": sf.Name, "label": sf.Label, "type": sf.Kind, "status": "stored-not-enforced"})
			}
		}
		objects = append(objects, map[string]any{"name": o.Name, "label": o.Label, "labelPlural": o.PluralLabel, "fields": fields})
	}
	return map[string]any{"Tenant": s.Tenant, "Revision": s.Revision, "Currency": s.Currency, "Objects": objects}
}

// ProtectTable installs one closed tenant policy. Owner gets the same policy;
// operator paths set tenant explicitly. Grants are conditional during legacy adoption.
func ProtectTable(ctx context.Context, tx pgx.Tx, schema, table string, readOnly bool) error {
	q := pgx.Identifier{schema, table}.Sanitize()
	sql := "ALTER TABLE " + q + " ENABLE ROW LEVEL SECURITY; ALTER TABLE " + q + " FORCE ROW LEVEL SECURITY; CREATE POLICY tenant_isolation ON " + q + " USING(tenant_id=nullif(current_setting('salescompel.tenant_id',true),'')) WITH CHECK(tenant_id=nullif(current_setting('salescompel.tenant_id',true),''))"
	if _, e := tx.Exec(ctx, sql); e != nil {
		return e
	}
	var role bool
	tx.QueryRow(ctx, "SELECT EXISTS(SELECT FROM pg_roles WHERE rolname='salescompel_records')").Scan(&role)
	if role {
		grant := "SELECT,INSERT,UPDATE,DELETE"
		if readOnly {
			grant = "SELECT"
		}
		_, e := tx.Exec(ctx, "GRANT "+grant+" ON "+q+" TO salescompel_records")
		return e
	}
	return nil
}
