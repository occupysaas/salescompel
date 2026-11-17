package records

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// Executor runs SQL in the caller's tenant transaction.
type Executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// closureSQL rebuilds the tenant's derived sharing tables ($1 tenant) from
// roles, groups, members and users (shr-05). It runs in the writing
// transaction, so a concurrent reader sees either the old or the new closure,
// never a mix: both statements commit together.
//
// user_groups: an active user is in the Role group of their role, the
// RoleAndSubordinates(Internal) groups of their role and every role above it,
// the Organization group, every Regular group naming them, and transitively
// every group naming one of those groups.
//
// user_principals: for each active user, themselves, their groups, every
// user (active or not) in a role strictly below theirs, and the groups of
// those subordinates (D34-25: an inactive user is in no group).
const closureSQL = `
DELETE FROM sc_sharing.user_groups WHERE tenant_id=$1;
DELETE FROM sc_sharing.user_principals WHERE tenant_id=$1;
WITH RECURSIVE
anc(role_id,ancestor_id) AS (
 SELECT id,id FROM sc_setup.user_roles WHERE tenant_id=$1
 UNION SELECT a.role_id,r.parent_role_id FROM anc a JOIN sc_setup.user_roles r ON r.tenant_id=$1 AND r.id=a.ancestor_id WHERE r.parent_role_id IS NOT NULL),
active AS (SELECT id,user_role_id FROM sc_setup.users WHERE tenant_id=$1 AND is_active),
base(user_id,group_id) AS (
 SELECT u.id,g.id FROM active u JOIN sc_setup.groups g ON g.tenant_id=$1 AND g.type='Role' AND g.related_id=u.user_role_id
 UNION SELECT u.id,g.id FROM active u JOIN anc ON anc.role_id=u.user_role_id
  JOIN sc_setup.groups g ON g.tenant_id=$1 AND g.type IN ('RoleAndSubordinates','RoleAndSubordinatesInternal') AND g.related_id=anc.ancestor_id
 UNION SELECT u.id,g.id FROM active u JOIN sc_setup.groups g ON g.tenant_id=$1 AND g.type='Organization'
 UNION SELECT u.id,m.group_id FROM active u JOIN sc_setup.group_members m ON m.tenant_id=$1 AND m.user_or_group_id=u.id),
nest(user_id,group_id) AS (
 SELECT user_id,group_id FROM base
 UNION SELECT n.user_id,m.group_id FROM nest n JOIN sc_setup.group_members m ON m.tenant_id=$1 AND m.user_or_group_id=n.group_id)
INSERT INTO sc_sharing.user_groups(tenant_id,user_id,group_id) SELECT $1,user_id,group_id FROM nest;
WITH RECURSIVE
anc(role_id,ancestor_id) AS (
 SELECT id,id FROM sc_setup.user_roles WHERE tenant_id=$1
 UNION SELECT a.role_id,r.parent_role_id FROM anc a JOIN sc_setup.user_roles r ON r.tenant_id=$1 AND r.id=a.ancestor_id WHERE r.parent_role_id IS NOT NULL),
active AS (SELECT id,user_role_id FROM sc_setup.users WHERE tenant_id=$1 AND is_active),
sub(user_id,sub_id) AS (
 SELECT u.id,s.id FROM active u JOIN anc ON anc.ancestor_id=u.user_role_id AND anc.role_id<>u.user_role_id
  JOIN sc_setup.users s ON s.tenant_id=$1 AND s.user_role_id=anc.role_id)
INSERT INTO sc_sharing.user_principals(tenant_id,user_id,principal_id)
 SELECT $1,id,id FROM active
 UNION SELECT $1,user_id,group_id FROM sc_sharing.user_groups WHERE tenant_id=$1
 UNION SELECT $1,user_id,sub_id FROM sub
 UNION SELECT $1,s.user_id,g.group_id FROM sub s JOIN sc_sharing.user_groups g ON g.tenant_id=$1 AND g.user_id=s.sub_id`

// RebuildSharing recomputes the tenant's closure tables. Call it in the same
// transaction as any change to roles, groups, members, User.UserRoleId or
// User.IsActive.
func RebuildSharing(ctx context.Context, db Executor, tenant string) error {
	// Parameters need one statement per call.
	for _, stmt := range strings.Split(closureSQL, ";\n") {
		if _, err := db.Exec(ctx, stmt, tenant); err != nil {
			return err
		}
	}
	return nil
}
