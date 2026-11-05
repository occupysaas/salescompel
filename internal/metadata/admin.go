package metadata

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ObjectInput struct{ Name, Label, PluralLabel string }
type FieldInput struct {
	Object, Name, Label, Type string
	Length                    int
}

var apiName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,39}__c$`)

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
func CreateObject(ctx context.Context, p *pgxpool.Pool, tenant string, in ObjectInput) (Object, error) {
	o := Object{}
	if !ValidAPIName(in.Name) || !validLabel(in.Label) || !validLabel(in.PluralLabel) {
		return o, errors.New("invalid object name or label")
	}
	e := ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		var revision int64
		if e := tx.QueryRow(ctx, "SELECT revision FROM sc_catalog.revisions WHERE tenant_id=$1 FOR UPDATE", tenant).Scan(&revision); e != nil {
			return errors.New("tenant is not provisioned")
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
		o = Object{ID: id, Tenant: tenant, Name: in.Name, Label: in.Label, PluralLabel: in.PluralLabel, Schema: "sc_records", Table: "o_" + id, Prefix: "a" + string(alphabet[n/62]) + string(alphabet[n%62]), Createable: true, Updateable: true, Deletable: true, Queryable: true, Fields: []Field{{Name: "Id", Type: "id", Column: "id"}, {Name: "Name", Label: "Name", Type: "string", Column: "name", Length: 80}}}
		if _, e := tx.Exec(ctx, "CREATE TABLE "+o.TableSQL()+"(tenant_id text NOT NULL REFERENCES public.tenants(id) ON DELETE CASCADE,id varchar(18) NOT NULL,name varchar(80) NOT NULL CHECK(length(trim(name))>0),PRIMARY KEY(tenant_id,id))"); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO sc_catalog.objects(id,tenant_id,api_name,label,plural_label,standard,physical_schema,physical_table,prefix) VALUES($1,$2,$3,$4,$5,false,$6,$7,$8)`, id, tenant, in.Name, in.Label, in.PluralLabel, o.Schema, o.Table, o.Prefix); e != nil {
			return e
		}
		for i, f := range o.Fields {
			if e := insertField(ctx, tx, tenant, id, f, i); e != nil {
				return e
			}
		}
		if e := ProtectTable(ctx, tx, o.Schema, o.Table, false); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, "UPDATE sc_catalog.revisions SET revision=revision+1 WHERE tenant_id=$1", tenant)
		return e
	})
	return o, e
}
func CreateField(ctx context.Context, p *pgxpool.Pool, tenant string, in FieldInput) (Field, error) {
	f := Field{}
	if !ValidAPIName(in.Name) || !validLabel(in.Label) || in.Type != "Text" || in.Length < 1 || in.Length > 255 {
		return f, errors.New("invalid field; only optional Text length 1-255 is supported")
	}
	e := ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		snapshot, e := Load(ctx, tx, tenant)
		if e != nil {
			return e
		}
		o, e := snapshot.Object(in.Object)
		if e != nil {
			return e
		}
		for _, old := range o.Fields {
			if strings.EqualFold(old.Name, in.Name) {
				return errors.New("field already exists")
			}
		}
		f = Field{ID: randomID(), Name: in.Name, Label: in.Label, Type: "string", Length: in.Length, Nillable: true, Createable: true, Updateable: true, ReferenceTo: []string{}}
		f.Column = "f_" + f.ID
		if _, e = tx.Exec(ctx, "ALTER TABLE "+o.TableSQL()+" ADD COLUMN "+pgx.Identifier{f.Column}.Sanitize()+fmt.Sprintf(" varchar(%d)", in.Length)); e != nil {
			return e
		}
		if e = insertField(ctx, tx, tenant, o.ID, f, len(o.Fields)); e != nil {
			return e
		}
		raw, _ := json.Marshal(f)
		if _, e = tx.Exec(ctx, "UPDATE sc_catalog.fields SET definition=$3 WHERE tenant_id=$1 AND object_id=$2 AND api_name=$4", tenant, o.ID, string(raw), f.Name); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, "UPDATE sc_catalog.revisions SET revision=revision+1 WHERE tenant_id=$1", tenant)
		return e
	})
	return f, e
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
