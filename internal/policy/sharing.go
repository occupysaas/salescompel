package policy

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/fieldtype"
	"salescompel/internal/metadata"
)

// Sharing is a request's record-level context (Plan 34): the tenant's
// internal org-wide defaults. Ownership, roles, groups, shares and rules are
// read by the access predicate in SQL, in the request's transaction, so they
// are never stale.
type Sharing struct {
	// owd maps a lower-cased object name to its stored internal access.
	owd map[string]string
}

// LoadSharing reads the tenant's org-wide default settings.
func LoadSharing(ctx context.Context, q Querier, tenant string) (*Sharing, error) {
	rows, e := q.Query(ctx, `SELECT o.api_name,s.internal_access FROM sc_sharing.object_settings s
		JOIN sc_catalog.objects o ON o.tenant_id=s.tenant_id AND o.id=s.object_id WHERE s.tenant_id=$1`, tenant)
	if e != nil {
		return nil, e
	}
	out := &Sharing{owd: map[string]string{}}
	var name, access string
	_, e = pgx.ForEachRow(rows, []any{&name, &access}, func() error {
		out.owd[strings.ToLower(name)] = access
		return nil
	})
	return out, e
}

// NewSharing builds a context from explicit org-wide defaults (tests).
func NewSharing(owd map[string]string) *Sharing {
	s := &Sharing{owd: map[string]string{}}
	for k, v := range owd {
		s.owd[strings.ToLower(k)] = v
	}
	return s
}

// Access is o's internal org-wide default (D34-16, D34-18). Without a
// stored setting it is Salesforce's new-org default: ControlledByParent for
// Contact and for master-detail details, Edit otherwise. A detail is always
// ControlledByParent. A nil context gives the defaults.
func (s *Sharing) Access(o metadata.Object) string {
	if _, ok := ControllingField(o); ok && !strings.EqualFold(o.Name, "Contact") {
		return metadata.AccessControlledByParent
	}
	if s != nil {
		if v, ok := s.owd[strings.ToLower(o.Name)]; ok {
			return v
		}
	}
	if strings.EqualFold(o.Name, "Contact") {
		return metadata.AccessControlledByParent
	}
	return metadata.AccessEdit
}

// ControllingField is the field whose parent controls o's access when o is
// ControlledByParent: Contact.AccountId, or a master-detail field.
func ControllingField(o metadata.Object) (metadata.Field, bool) {
	for _, f := range o.Fields {
		if f.Options.Kind == fieldtype.MasterDetail && len(f.ReferenceTo) == 1 {
			return f, true
		}
	}
	if strings.EqualFold(o.Name, "Contact") {
		for _, f := range o.Fields {
			if f.Name == "AccountId" {
				return f, true
			}
		}
	}
	return metadata.Field{}, false
}

// Bypass reports whether p reaches every record of object at the level
// through View All / Modify All (shr-09). write is Edit or Delete.
func (p Permissions) Bypass(object string, write bool) bool {
	a := p.Object(object)
	if write {
		return p.system.ModifyAllData || a.ModifyAll
	}
	return p.system.ModifyAllData || p.system.ViewAllData || a.ViewAll || a.ModifyAll
}
