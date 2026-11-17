package soql

import (
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

// Level is the record access a statement needs (Plan 34 spec, Enforcement).
type Level int

const (
	LevelRead Level = iota
	LevelEdit
	LevelDelete
)

// SharingClause returns a SQL boolean admitting the rows of alias (a row of
// obj) that p may reach at level, or "" when every row is admitted. The
// clause reads ownership, the closure tables, manual shares and owner rules
// in the caller's transaction (spec "The access predicate"). addArg appends
// a parameter and returns its number, continuing the caller's numbering.
// spec: shr-10, shr-11
func SharingClause(p *policy.Principal, alias string, obj metadata.Object, level Level, addArg func(any) int) string {
	if p == nil {
		return "FALSE"
	}
	s := &sharer{p: p, addArg: addArg}
	return s.clause(alias, obj, level, 0)
}

type sharer struct {
	p      *policy.Principal
	addArg func(any) int
	user   string
}

func (s *sharer) userParam() string {
	if s.user == "" {
		s.user = "$" + itoa(s.addArg(s.p.User))
	}
	return s.user
}

func (s *sharer) principals(alias string) string {
	return "(SELECT up.principal_id FROM sc_sharing.user_principals up WHERE up.tenant_id=" + alias + ".tenant_id AND up.user_id=" + s.userParam() + ")"
}

func levels(level Level) string {
	if level == LevelRead {
		return "('Read','Edit')"
	}
	return "('Edit')"
}

func (s *sharer) object(name string) (metadata.Object, bool) {
	if s.p.Catalog == nil {
		return metadata.Object{}, false
	}
	o, err := s.p.Catalog.Object(name)
	return o, err == nil
}

func column(o metadata.Object, field string) string {
	for _, f := range o.Fields {
		if f.Name == field {
			return pgx.Identifier{f.Column}.Sanitize()
		}
	}
	return ""
}

func (s *sharer) clause(alias string, obj metadata.Object, level Level, depth int) string {
	if depth > 4 {
		return "FALSE"
	}
	if parent, ok := metadata.ShareParent(obj.Name); ok {
		// A share row is visible when its record is readable (D34-06).
		po, ok := s.object(parent)
		col := column(obj, parent+"Id")
		if !ok || col == "" {
			return "FALSE"
		}
		return s.parent(alias, col, po, LevelRead, depth)
	}
	if metadata.IsSetupObject(obj.Name) || column(obj, "OwnerId") == "" {
		return "" // setup objects keep Plan 33's rules; they have no owner
	}
	if s.p.Perms.Bypass(obj.Name, level != LevelRead) {
		return "" // shr-09
	}
	switch owd := s.p.Sharing.Access(obj); owd {
	case metadata.AccessControlledByParent:
		f, ok := policy.ControllingField(obj)
		po, found := s.object(firstOf(f.ReferenceTo))
		if !ok || !found {
			return "FALSE"
		}
		col := alias + "." + pgx.Identifier{f.Column}.Sanitize()
		parent := s.parent(alias, pgx.Identifier{f.Column}.Sanitize(), po, level, depth)
		if parent == "" {
			parent = col + " IS NOT NULL"
		}
		// A contact with no Account is private to its owner (D34-23).
		return "(" + parent + " OR (" + col + " IS NULL AND " + alias + ".owner_id=" + s.userParam() + "))"
	case metadata.AccessEdit:
		if level != LevelDelete {
			return ""
		}
	case metadata.AccessRead:
		if level == LevelRead {
			return ""
		}
	case metadata.AccessNone:
	default:
		return "FALSE" // shr-12: an OWD the candidate can't evaluate grants nothing
	}
	terms := s.terms(alias, obj, level)
	if strings.EqualFold(obj.Name, "Account") && level == LevelRead {
		terms = append(terms, s.implicitParent(alias))
	}
	if strings.EqualFold(obj.Name, "Contact") {
		terms = append(terms, s.accountOwnerChild(alias, obj, level)...)
	}
	return "(" + strings.Join(terms, " OR ") + ")"
}

// terms are ownership (with the hierarchy), manual shares and owner rules.
// Delete needs owner-level access: ownership or the hierarchy only.
func (s *sharer) terms(alias string, obj metadata.Object, level Level) []string {
	out := []string{alias + ".owner_id IN " + s.principals(alias)}
	if level == LevelDelete {
		return out
	}
	out = append(out,
		"EXISTS(SELECT 1 FROM sc_sharing.shares sh WHERE sh.tenant_id="+alias+".tenant_id AND sh.record_id="+alias+".id AND sh.access_level IN "+levels(level)+" AND sh.user_or_group_id IN "+s.principals(alias)+")",
		"EXISTS(SELECT 1 FROM sc_sharing.owner_rules ru JOIN sc_sharing.user_groups src ON src.tenant_id=ru.tenant_id AND src.group_id=ru.source_group_id AND src.user_id="+alias+".owner_id"+
			" WHERE ru.tenant_id="+alias+".tenant_id AND ru.object_id=$"+itoa(s.addArg(obj.ID))+" AND ru.access_level IN "+levels(level)+" AND ru.target_group_id IN "+s.principals(alias)+")")
	return out
}

// parent admits alias when its parent row (column col of alias) is
// admitted at level.
func (s *sharer) parent(alias, col string, po metadata.Object, level Level, depth int) string {
	pa := "sp" + itoa(depth+1)
	c := s.clause(pa, po, level, depth+1)
	if c == "" {
		return ""
	}
	return "EXISTS(SELECT 1 FROM " + po.TableSQL() + " " + pa + " WHERE " + pa + ".tenant_id=" + alias + ".tenant_id AND " + pa + ".id=" + alias + "." + col + " AND " + c + ")"
}

// implicitParent gives Read on an Account to users who reach one of its
// Contacts through ownership, the hierarchy, a share or a rule (D34-21).
// View All on Contact doesn't count, and nothing is implied while Contact is
// ControlledByParent.
func (s *sharer) implicitParent(alias string) string {
	contact, ok := s.object("Contact")
	col := column(contact, "AccountId")
	if !ok || col == "" || s.p.Sharing.Access(contact) == metadata.AccessControlledByParent {
		return "FALSE"
	}
	return "EXISTS(SELECT 1 FROM " + contact.TableSQL() + " ic WHERE ic.tenant_id=" + alias + ".tenant_id AND ic." + col + "=" + alias + ".id AND (" + strings.Join(s.terms("ic", contact, LevelRead), " OR ") + "))"
}

// accountOwnerChild gives a Private Contact's access to the parent
// Account's owner (and their managers) at the owner's role setting, and to
// grantees of an AccountShare whose ContactAccessLevel covers level
// (D34-22, D34-06).
func (s *sharer) accountOwnerChild(alias string, contact metadata.Object, level Level) []string {
	account, ok := s.object("Account")
	col := column(contact, "AccountId")
	if !ok || col == "" || level == LevelDelete {
		return nil
	}
	roleLevel := ""
	if level == LevelEdit {
		roleLevel = " AND ro.contact_access_for_account_owner='Edit'"
	}
	return []string{
		"EXISTS(SELECT 1 FROM " + account.TableSQL() + " ia JOIN sc_setup.users ow ON ow.tenant_id=ia.tenant_id AND ow.id=ia.owner_id" +
			" LEFT JOIN sc_setup.user_roles ro ON ro.tenant_id=ow.tenant_id AND ro.id=ow.user_role_id" +
			" WHERE ia.tenant_id=" + alias + ".tenant_id AND ia.id=" + alias + "." + col + " AND ia.owner_id IN " + s.principals(alias) + roleLevel + ")",
		"EXISTS(SELECT 1 FROM sc_sharing.shares cs WHERE cs.tenant_id=" + alias + ".tenant_id AND cs.record_id=" + alias + "." + col +
			" AND cs.contact_access_level IN " + levels(level) + " AND cs.user_or_group_id IN " + s.principals(alias) + ")",
	}
}

func firstOf(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// shareAnd is " AND <predicate>" for alias's rows of obj at Read, or "".
func (c *compiler) shareAnd(alias string, obj metadata.Object) string {
	if c.access == nil {
		return ""
	}
	if s := SharingClause(c.access, alias, obj, LevelRead, c.addArg); s != "" {
		return " AND " + s
	}
	return ""
}
