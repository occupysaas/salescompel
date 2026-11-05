package platform

import (
	"context"
	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
	"strings"
	"testing"
)

func TestDynamicRecords(t *testing.T) {
	s := testStore(t)
	p, other := principals(t, s)
	ctx := context.Background()
	o, e := metadata.CreateObject(ctx, s.pool, p.Tenant, metadata.ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	defer s.pool.Exec(ctx, "DROP TABLE "+o.TableSQL())
	if _, e = metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{Object: "Project__c", Name: "Notes__c", Label: "Notes", Type: "Text", Length: 80}); e != nil {
		t.Fatal(e)
	}
	id, e := s.Create(ctx, p, "Project__c", map[string]any{"Name": "First", "Notes__c": "hello"})
	if e != nil {
		t.Fatal(e)
	}
	row, e := s.Read(ctx, p, "Project__c", id)
	if e != nil || row["Notes__c"] != "hello" {
		t.Fatal(row, e)
	}
	if _, e = s.Read(ctx, other, "Project__c", id); e == nil {
		t.Fatal("cross tenant custom read")
	}
	if e = s.Update(ctx, p, "Project__c", id, map[string]any{"Notes__c": strings.Repeat("x", 81)}); e == nil {
		t.Fatal("oversize accepted")
	}
	if e = s.Update(ctx, p, "Project__c", id, map[string]any{"Notes__c": ""}); e != nil {
		t.Fatal(e)
	}
	row, e = s.Read(ctx, p, "Project__c", id)
	if e != nil || row["Notes__c"] != nil {
		t.Fatal(row, e)
	}
	q, e := ParseQuery("SELECT Id,Name,Notes__c FROM project__c ORDER BY Name LIMIT 10")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := s.Select(ctx, p, q)
	if e != nil || len(rows) != 1 || rows[0]["Name"] != "First" {
		t.Fatal(rows, e)
	}
}
func TestDynamicCompatKeyIsOrdinaryText(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	o, e := metadata.CreateObject(ctx, s.pool, p.Tenant, metadata.ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	defer s.pool.Exec(ctx, "DROP TABLE "+o.TableSQL())
	for _, object := range []string{"Contact", "Project__c"} {
		f, e := metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{Object: object, Name: "Compat_Key__c", Label: "Project key", Type: "Text", Length: 80})
		if e != nil {
			t.Fatal(e)
		}
		if object == "Contact" {
			defer s.pool.Exec(ctx, "ALTER TABLE public.contacts DROP COLUMN "+pgx.Identifier{f.Column}.Sanitize())
		}
		data := map[string]any{"Compat_Key__c": "  Mixed Key  "}
		if object == "Contact" {
			data["LastName"] = "Contact"
		} else {
			data["Name"] = "Project"
		}
		id, e := s.Create(ctx, p, object, data)
		if e != nil {
			t.Fatal(object, e)
		}
		row, e := s.Read(ctx, p, object, id)
		if e != nil || row["Compat_Key__c"] != "  Mixed Key  " {
			t.Fatal(row, e)
		}
		if e = s.Update(ctx, p, object, id, map[string]any{"Compat_Key__c": "changed"}); e != nil {
			t.Fatal(e)
		}
	}
}
