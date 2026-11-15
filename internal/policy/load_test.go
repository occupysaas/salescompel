package policy

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/metadata"
	"salescompel/internal/migration"
	"salescompel/internal/recordid"
)

// loadFixture migrates the test database, seeds a fresh tenant and creates
// one Standard User. It returns the owner pool, tenant and user ID.
func loadFixture(t *testing.T) (*pgxpool.Pool, string, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL required in CI")
		}
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration")
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	if e = migration.Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	tenant := fmt.Sprintf("load%d", time.Now().UnixNano())
	user := newTestID(t, "005")
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, "INSERT INTO public.tenants(id) VALUES($1)", tenant); e != nil {
			return e
		}
		if e := metadata.SeedTenant(ctx, tx, tenant); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, "INSERT INTO public.users(id,tenant_id,username,username_canonical,password_hash,role) VALUES($1,$2,$3,$3,'','restricted')", user, tenant, tenant+"@example.invalid"); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, `INSERT INTO sc_setup.users(id,tenant_id,username,username_canonical,last_name,alias,email,profile_id,time_zone_sid_key,locale_sid_key,language_locale_key,email_encoding_key)
			SELECT $1,$2,$3,$3,'User','user',$3,id,'America/Los_Angeles','en_US','en_US','UTF-8' FROM sc_setup.profiles WHERE tenant_id=$2 AND name='Standard User'`, user, tenant, tenant+"@example.invalid"); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, `INSERT INTO sc_setup.permission_set_assignments(id,tenant_id,assignee_id,permission_set_id)
			SELECT $1,$2,$3,permission_set_id FROM sc_setup.profiles WHERE tenant_id=$2 AND name='Standard User'`, newTestID(t, "0Pa"), tenant, user)
		return e
	})
	t.Cleanup(func() { p.Exec(context.Background(), "DELETE FROM public.tenants WHERE id=$1", tenant) })
	return p, tenant, user
}

func newTestID(t *testing.T, prefix string) string {
	t.Helper()
	id, e := recordid.New(prefix)
	if e != nil {
		t.Fatal(e)
	}
	return id
}

func inTenantTx(t *testing.T, p *pgxpool.Pool, tenant string, fn func(pgx.Tx) error) {
	t.Helper()
	ctx := context.Background()
	tx, e := p.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant); e != nil {
		t.Fatal(e)
	}
	if e = fn(tx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
}

func load(t *testing.T, p *pgxpool.Pool, tenant, user string) (Permissions, int64) {
	t.Helper()
	var perms Permissions
	var rev int64
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		var e error
		perms, rev, e = LoadPermissions(context.Background(), tx, tenant, user)
		return e
	})
	return perms, rev
}

func compatText() metadata.Field {
	for _, f := range metadata.StandardObjects()[0].Fields {
		if f.Name == "Compat_Text__c" {
			return f
		}
	}
	panic("Account.Compat_Text__c missing")
}

const grantCompatText = `INSERT INTO sc_setup.field_permissions(id,tenant_id,parent_id,sobject_type,field,permissions_read) VALUES($1,$2,$3,'Account','Account.Compat_Text__c',true)`

func TestLoadPermissionsUnionsProfileAndAssigned(t *testing.T) {
	p, tenant, user := loadFixture(t)
	before, _ := load(t, p, tenant, user)
	if before.FieldRead("Account", compatText()) || !before.Object("Account").Edit || !before.System().APIEnabled {
		t.Fatalf("Standard User baseline wrong: %+v %+v", before.Object("Account"), before.System())
	}
	set := newTestID(t, "0PS")
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		ctx := context.Background()
		if _, e := tx.Exec(ctx, "INSERT INTO sc_setup.permission_sets(id,tenant_id,name,label) VALUES($1,$2,'Compat_Reader','Compat Reader')", set, tenant); e != nil {
			return e
		}
		if _, e := tx.Exec(ctx, grantCompatText, newTestID(t, "01k"), tenant, set); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, "INSERT INTO sc_setup.permission_set_assignments(id,tenant_id,assignee_id,permission_set_id) VALUES($1,$2,$3,$4)", newTestID(t, "0Pa"), tenant, user, set)
		return e
	})
	after, _ := load(t, p, tenant, user)
	if !after.FieldRead("Account", compatText()) || after.FieldEdit("Account", compatText()) || !after.Object("Account").Delete {
		t.Fatal("assigned set did not union with the profile")
	}
}

func TestPermissionRevisionInvalidatesCache(t *testing.T) {
	p, tenant, user := loadFixture(t)
	var cache Cache
	perms, rev := load(t, p, tenant, user)
	cache.Put(tenant, user, rev, perms)
	if _, ok := cache.Get(tenant, user, rev); !ok {
		t.Fatal("cache missed at the stored revision")
	}
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		ctx := context.Background()
		if _, e := tx.Exec(ctx, grantCompatText, newTestID(t, "01k"), tenant, profileSet(t, tx, tenant, "Standard User")); e != nil {
			return e
		}
		_, e := tx.Exec(ctx, "UPDATE sc_setup.revisions SET revision=revision+1 WHERE tenant_id=$1", tenant)
		return e
	})
	reloaded, current := load(t, p, tenant, user)
	if current != rev+1 {
		t.Fatalf("revision %d after bump from %d", current, rev)
	}
	if _, ok := cache.Get(tenant, user, current); ok {
		t.Fatal("cache served permissions from an older revision")
	}
	if !reloaded.FieldRead("Account", compatText()) {
		t.Fatal("reload did not show the new grant")
	}
}

func profileSet(t *testing.T, tx pgx.Tx, tenant, profile string) string {
	t.Helper()
	var id string
	if e := tx.QueryRow(context.Background(), "SELECT permission_set_id FROM sc_setup.profiles WHERE tenant_id=$1 AND name=$2", tenant, profile).Scan(&id); e != nil {
		t.Fatal(e)
	}
	return id
}

func TestInactiveUserHasNoPermissions(t *testing.T) {
	p, tenant, user := loadFixture(t)
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		_, e := tx.Exec(context.Background(), "UPDATE sc_setup.users SET is_active=false WHERE tenant_id=$1 AND id=$2", tenant, user)
		return e
	})
	perms, _ := load(t, p, tenant, user)
	if perms.System() != (SystemPerms{}) || perms.Object("Account") != (ObjectAccess{}) || perms.Object("Contact") != (ObjectAccess{}) || perms.Object("User").Read {
		t.Fatalf("inactive user kept access: %+v %+v", perms.System(), perms.Object("Account"))
	}
	if missing, _ := load(t, p, tenant, newTestID(t, "005")); missing.Object("Account") != (ObjectAccess{}) {
		t.Fatal("unknown user has access")
	}
}

func TestProvisionedRoleKeepsAccess(t *testing.T) {
	p, tenant, user := loadFixture(t)
	standard, _ := load(t, p, tenant, user)
	var restricted, admin Permissions
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		var e error
		if restricted, _, e = RoleProfilePermissions(context.Background(), tx, tenant, "restricted"); e != nil {
			return e
		}
		admin, _, e = RoleProfilePermissions(context.Background(), tx, tenant, "admin")
		return e
	})
	if !reflect.DeepEqual(restricted, standard) {
		t.Fatalf("restricted role differs from Standard User:\n%+v\n%+v", restricted, standard)
	}
	if s := admin.System(); !(s.ModifyAllData && s.ViewAllData && s.APIEnabled && s.ManageUsers) || !admin.FieldRead("Account", compatText()) {
		t.Fatalf("admin role lacks System Administrator: %+v", s)
	}
	inTenantTx(t, p, tenant, func(tx pgx.Tx) error {
		if _, _, e := RoleProfilePermissions(context.Background(), tx, tenant, "unknown"); e == nil {
			t.Error("unknown role accepted")
		}
		return nil
	})
}
