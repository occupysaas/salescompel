package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/recordid"
)

// Operator sharing settings (Plan 34, D34-16, D34-30): org-wide defaults and
// owner-based sharing rules, until Plan 35 deploys them as metadata and a
// setup UI exists.

// SharingRule is one owner-based rule as the operator sees it.
type SharingRule struct {
	ID, Object, Name, Source, Target, Access, ContactAccess string
}

func (s *Store) tenantTx(ctx context.Context, tenant string, fn func(pgx.Tx, metadata.Snapshot) error) error {
	if tenant == "" {
		return errors.New("tenant required")
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant); e != nil {
		return e
	}
	snap, e := metadata.Load(ctx, tx, tenant)
	if e != nil {
		return e
	}
	if e = fn(tx, snap); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// recordObject resolves an object that carries owners.
func recordObject(snap metadata.Snapshot, name string) (metadata.Object, error) {
	o, e := snap.Object(name)
	if e != nil || metadata.IsSetupObject(o.Name) {
		return metadata.Object{}, fmt.Errorf("unknown record object %q", name)
	}
	return o, nil
}

// SetDefaultAccess sets an object's internal org-wide default.
func (s *Store) SetDefaultAccess(ctx context.Context, tenant, object, access string) error {
	return s.tenantTx(ctx, tenant, func(tx pgx.Tx, snap metadata.Snapshot) error {
		o, e := recordObject(snap, object)
		if e != nil {
			return e
		}
		allowed := map[string]bool{metadata.AccessNone: true, metadata.AccessRead: true, metadata.AccessEdit: true}
		if strings.EqualFold(o.Name, "Contact") {
			allowed[metadata.AccessControlledByParent] = true
		}
		if f, ok := policy.ControllingField(o); ok && f.Options.Kind.Reference() && !strings.EqualFold(o.Name, "Contact") {
			return fmt.Errorf("%s is a master-detail detail and is always ControlledByParent", o.Name)
		}
		if !allowed[access] {
			return fmt.Errorf("%s doesn't accept org-wide default %q", o.Name, access)
		}
		current, e := policy.LoadSharing(ctx, tx, tenant)
		if e != nil {
			return e
		}
		// Contact can't be more open than a Private Account (D34-16).
		account, _ := snap.Object("Account")
		contact, _ := snap.Object("Contact")
		accountAccess, contactAccess := current.Access(account), current.Access(contact)
		switch {
		case strings.EqualFold(o.Name, "Account"):
			accountAccess = access
		case strings.EqualFold(o.Name, "Contact"):
			contactAccess = access
		}
		if accountAccess == metadata.AccessNone && (contactAccess == metadata.AccessRead || contactAccess == metadata.AccessEdit) {
			return errors.New("while Account is Private, Contact must be Private or ControlledByParent")
		}
		_, e = tx.Exec(ctx, `INSERT INTO sc_sharing.object_settings(tenant_id,object_id,internal_access) VALUES($1,$2,$3)
			ON CONFLICT(tenant_id,object_id) DO UPDATE SET internal_access=excluded.internal_access`, tenant, o.ID, access)
		return e
	})
}

// groupID accepts a Group ID or a Regular group's developer name.
func groupID(ctx context.Context, tx pgx.Tx, tenant, v string) (string, error) {
	var id string
	e := tx.QueryRow(ctx, `SELECT id FROM sc_setup.groups WHERE tenant_id=$1 AND (id=$2 OR (type='Regular' AND lower(developer_name)=lower($2)))`, tenant, v).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", fmt.Errorf("unknown group %q", v)
	}
	return id, e
}

// AddSharingRule adds an owner-based rule and returns its ID.
func (s *Store) AddSharingRule(ctx context.Context, tenant string, r SharingRule) (string, error) {
	var id string
	e := s.tenantTx(ctx, tenant, func(tx pgx.Tx, snap metadata.Snapshot) error {
		o, e := recordObject(snap, r.Object)
		if e != nil {
			return e
		}
		if r.Name == "" || len(r.Name) > 80 {
			return errors.New("rule name required (at most 80 characters)")
		}
		if r.Access != metadata.AccessRead && r.Access != metadata.AccessEdit {
			return errors.New("rule access must be Read or Edit")
		}
		if r.ContactAccess != "" && (!strings.EqualFold(o.Name, "Account") || r.ContactAccess != "None" && r.ContactAccess != "Read" && r.ContactAccess != "Edit") {
			return errors.New("contact access applies to Account rules and is None, Read or Edit")
		}
		source, e := groupID(ctx, tx, tenant, r.Source)
		if e != nil {
			return e
		}
		target, e := groupID(ctx, tx, tenant, r.Target)
		if e != nil {
			return e
		}
		if id, e = recordid.New("02c"); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO sc_sharing.owner_rules(tenant_id,id,object_id,name,source_group_id,target_group_id,access_level,contact_access_level)
			VALUES($1,$2,$3,$4,$5,$6,$7,nullif($8,''))`, tenant, id, o.ID, r.Name, source, target, r.Access, r.ContactAccess)
		return e
	})
	return id, e
}

// SharingRules lists the tenant's owner-based rules by object and name.
func (s *Store) SharingRules(ctx context.Context, tenant string) ([]SharingRule, error) {
	var out []SharingRule
	e := s.tenantTx(ctx, tenant, func(tx pgx.Tx, _ metadata.Snapshot) error {
		rows, e := tx.Query(ctx, `SELECT r.id,o.api_name,r.name,r.source_group_id,r.target_group_id,r.access_level,coalesce(r.contact_access_level,'')
			FROM sc_sharing.owner_rules r JOIN sc_catalog.objects o ON o.tenant_id=r.tenant_id AND o.id=r.object_id
			WHERE r.tenant_id=$1 ORDER BY o.api_name,r.name`, tenant)
		if e != nil {
			return e
		}
		out, e = pgx.CollectRows(rows, func(row pgx.CollectableRow) (SharingRule, error) {
			var r SharingRule
			return r, row.Scan(&r.ID, &r.Object, &r.Name, &r.Source, &r.Target, &r.Access, &r.ContactAccess)
		})
		return e
	})
	return out, e
}

// RemoveSharingRule deletes a rule by ID.
func (s *Store) RemoveSharingRule(ctx context.Context, tenant, id string) error {
	return s.tenantTx(ctx, tenant, func(tx pgx.Tx, _ metadata.Snapshot) error {
		tag, e := tx.Exec(ctx, "DELETE FROM sc_sharing.owner_rules WHERE tenant_id=$1 AND id=$2", tenant, id)
		if e == nil && tag.RowsAffected() == 0 {
			e = fmt.Errorf("unknown rule %q", id)
		}
		return e
	})
}
