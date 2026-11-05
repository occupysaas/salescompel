package metadata_test

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	. "salescompel/internal/metadata"
	"salescompel/internal/migration"
	"strings"
	"testing"
	"time"
)

func catalogPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	p, e := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	if e = migration.Up(ctx, p); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestSchemaInputRules(t *testing.T) {
	for _, name := range []string{"__c", "X___c", "A__B__c", "A'__c", "é__c", strings.Repeat("A", 41) + "__c"} {
		if ValidAPIName(name) {
			t.Fatalf("accepted %q", name)
		}
	}
	for _, name := range []string{"X__c", strings.Repeat("A", 40) + "__c", "Project_Notes__c"} {
		if !ValidAPIName(name) {
			t.Fatalf("rejected %q", name)
		}
	}
}
func TestTenantCatalogNames(t *testing.T) {
	ctx := context.Background()
	p := catalogPool(t)
	tenant := fmt.Sprintf("catalog%d", time.Now().UnixNano())
	p.Exec(ctx, "INSERT INTO tenants(id) VALUES($1)", tenant)
	defer p.Exec(ctx, "DELETE FROM tenants WHERE id=$1", tenant)
	tx, _ := p.Begin(ctx)
	if e := SeedTenant(ctx, tx, tenant); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	obj, e := CreateObject(ctx, p, tenant, ObjectInput{"Project__c", "Project", "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Exec(ctx, "DROP TABLE "+obj.TableSQL())
	f, e := CreateField(ctx, p, tenant, FieldInput{"Project__c", "Notes__c", "Notes", "Text", 80})
	if e != nil || f.Length != 80 {
		t.Fatal(f, e)
	}
	if _, e = CreateObject(ctx, p, tenant, ObjectInput{"project__c", "Other", "Others"}); e == nil {
		t.Fatal("duplicate accepted")
	}
	tx, _ = p.Begin(ctx)
	defer tx.Rollback(ctx)
	snapshot, e := Load(ctx, tx, tenant)
	if e != nil {
		t.Fatal(e)
	}
	project, e := snapshot.Object("pRoJeCt__c")
	if e != nil || project.Name != "Project__c" || len(project.Fields) != 3 {
		t.Fatal(project, e)
	}
	if snapshot.Revision != 3 {
		t.Fatal(snapshot.Revision)
	}
}
