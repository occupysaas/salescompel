package platform

import (
	"context"
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
