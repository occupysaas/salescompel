package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"salescompel/internal/metadata"
	"sort"
	"strings"
	"sync"
	"testing"
)

// typedFixture holds a tenant with one field of every active kind on Item__c,
// plus Parent__c (lookup targets) and Child__c/Grandchild__c (master-detail).
type typedFixture struct {
	s     *Store
	p     Principal
	other Principal
	cols  map[string]string
	items metadata.Object
}

func newTypedFixture(t *testing.T) typedFixture {
	t.Helper()
	s := testStore(t)
	p, other := principals(t, s)
	ctx := context.Background()
	f := typedFixture{s: s, p: p, other: other, cols: map[string]string{}}
	for _, o := range []metadata.ObjectInput{
		{Name: "Parent__c", Label: "Parent", PluralLabel: "Parents"},
		{Name: "Item__c", Label: "Item", PluralLabel: "Items", NameType: "AutoNumber", DisplayFormat: "IT-{0000}", StartingNumber: 1},
		{Name: "Child__c", Label: "Child", PluralLabel: "Children"},
		{Name: "Grandchild__c", Label: "Grandchild", PluralLabel: "Grandchildren"},
	} {
		obj, e := metadata.CreateObject(ctx, s.pool, p.Tenant, o)
		if e != nil {
			t.Fatal(e)
		}
		if o.Name == "Item__c" {
			f.items = obj
		}
		t.Cleanup(func() { s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+obj.TableSQL()+" CASCADE") })
	}
	def := func(v string) *string { return &v }
	vals := func(v ...string) []metadata.PicklistValue {
		out := []metadata.PicklistValue{}
		for _, x := range v {
			out = append(out, metadata.PicklistValue{Value: x, Label: x, Active: true})
		}
		return out
	}
	for _, in := range []metadata.FieldInput{
		{Object: "Item__c", Name: "Txt__c", Type: "Text", Length: 10},
		{Object: "Item__c", Name: "Area__c", Type: "TextArea"},
		{Object: "Item__c", Name: "Long__c", Type: "LongTextArea", Length: 300},
		{Object: "Item__c", Name: "Num__c", Type: "Number", Precision: 18, Scale: 2},
		{Object: "Item__c", Name: "Small__c", Type: "Number", Precision: 5, Scale: 2},
		{Object: "Item__c", Name: "Amount__c", Type: "Currency"},
		{Object: "Item__c", Name: "Pct__c", Type: "Percent", Precision: 5, Scale: 2},
		{Object: "Item__c", Name: "Day__c", Type: "Date"},
		{Object: "Item__c", Name: "At__c", Type: "DateTime"},
		{Object: "Item__c", Name: "Flag__c", Type: "Checkbox", Default: def("true")},
		{Object: "Item__c", Name: "Stage__c", Type: "Picklist", Values: vals("Open", "Closed"), Restricted: true, Default: def("Open")},
		{Object: "Item__c", Name: "Tag__c", Type: "Picklist", Values: vals("A")},
		{Object: "Item__c", Name: "Multi__c", Type: "MultiselectPicklist", Values: vals("A", "B", "C")},
		{Object: "Item__c", Name: "Mail__c", Type: "Email"},
		{Object: "Item__c", Name: "Tel__c", Type: "Phone"},
		{Object: "Item__c", Name: "Site__c", Type: "Url"},
		{Object: "Item__c", Name: "Parent__c", Type: "Lookup", ReferenceTo: "Parent__c"},
		{Object: "Item__c", Name: "Keep__c", Type: "Lookup", ReferenceTo: "Parent__c", DeleteBehavior: "Restrict"},
		{Object: "Item__c", Name: "Serial__c", Type: "AutoNumber", DisplayFormat: "S-{000}", StartingNumber: 7},
		{Object: "Child__c", Name: "Parent__c", Type: "MasterDetail", ReferenceTo: "Parent__c"},
		{Object: "Grandchild__c", Name: "Child__c", Type: "MasterDetail", ReferenceTo: "Child__c"},
	} {
		in.Label = in.Name
		fd, e := metadata.CreateField(ctx, s.pool, p.Tenant, in)
		if e != nil {
			t.Fatalf("%s.%s: %v", in.Object, in.Name, e)
		}
		f.cols[in.Object+"."+in.Name] = fd.Column
	}
	return f
}

// stored returns a column's value as PostgreSQL text ("<null>" for NULL).
func (f typedFixture) stored(t *testing.T, field, id string) string {
	t.Helper()
	var v string
	col := f.cols["Item__c."+field]
	if field == "Name" {
		col = "name"
	}
	if e := f.s.pool.QueryRow(context.Background(), "SELECT coalesce("+`"`+col+`"`+"::text,'<null>') FROM "+f.items.TableSQL()+" WHERE id=$1", id).Scan(&v); e != nil {
		t.Fatal(e)
	}
	return v
}

func apiCode(err error) (string, []string) {
	var api *APIError
	if errors.As(err, &api) {
		return api.Code, api.Fields
	}
	return "", nil
}

func TestWriteEveryKind(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	parent, e := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "P"})
	if e != nil {
		t.Fatal(e)
	}
	id, e := f.s.Create(ctx, f.p, "Item__c", map[string]any{
		"Txt__c": "hello", "Area__c": "line1\nline2", "Long__c": strings.Repeat("x", 300),
		"Num__c": json.Number("0.30000000000000004"), "Amount__c": "12.5", "Pct__c": json.Number("12.345"),
		"Day__c": "2026-11-11", "At__c": "2026-03-08T23:30:00-05:00",
		"Multi__c": " C ;A;;C", "Mail__c": "a@b.co", "Tel__c": "+1 (555) 0100", "Site__c": "example.com/x",
		"Parent__c": parent[:15],
	})
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]string{
		"Name": "IT-0001", "Serial__c": "S-007", "Txt__c": "hello", "Area__c": "line1\nline2",
		"Num__c": "0.30", "Amount__c": "12.50", "Pct__c": "12.35", "Day__c": "2026-11-11",
		"Flag__c": "true", "Stage__c": "Open", "Tag__c": "<null>", "Multi__c": "A;C",
		"Mail__c": "a@b.co", "Tel__c": "+1 (555) 0100", "Site__c": "example.com/x", "Parent__c": parent,
	}
	for field, w := range want {
		if got := f.stored(t, field, id); got != w {
			t.Errorf("%s = %q want %q", field, got, w)
		}
	}
	var at string
	f.s.pool.QueryRow(ctx, "SELECT to_char(\""+f.cols["Item__c.At__c"]+"\" AT TIME ZONE 'UTC','YYYY-MM-DD\"T\"HH24:MI:SS') FROM "+f.items.TableSQL()+" WHERE id=$1", id).Scan(&at)
	if at != "2026-03-09T04:30:00" {
		t.Errorf("At__c = %q", at)
	}
	second, e := f.s.Create(ctx, f.p, "Item__c", map[string]any{"Flag__c": nil, "Stage__c": "Closed", "Num__c": ""})
	if e != nil {
		t.Fatal(e)
	}
	if f.stored(t, "Flag__c", second) != "false" || f.stored(t, "Stage__c", second) != "Closed" || f.stored(t, "Num__c", second) != "<null>" || f.stored(t, "Name", second) != "IT-0002" || f.stored(t, "Serial__c", second) != "S-008" {
		t.Error("explicit null checkbox, picklist override, blank number or numbering wrong")
	}
	if e = f.s.Update(ctx, f.p, "Item__c", second, map[string]any{"Tag__c": "Unlisted value", "Multi__c": ""}); e != nil {
		t.Fatal(e)
	}
	if f.stored(t, "Tag__c", second) != "Unlisted value" || f.stored(t, "Multi__c", second) != "<null>" || f.stored(t, "Stage__c", second) != "Closed" {
		t.Error("unrestricted picklist, empty multi-select or update defaults wrong")
	}
}

func TestWriteErrors(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	otherParent, e := func() (string, error) {
		obj, e := metadata.CreateObject(ctx, f.s.pool, f.other.Tenant, metadata.ObjectInput{Name: "Parent__c", Label: "Parent", PluralLabel: "Parents"})
		if e != nil {
			return "", e
		}
		t.Cleanup(func() { f.s.pool.Exec(context.Background(), "DROP TABLE IF EXISTS "+obj.TableSQL()+" CASCADE") })
		return f.s.Create(ctx, f.other, "Parent__c", map[string]any{"Name": "foreign"})
	}()
	if e != nil {
		t.Fatal(e)
	}
	item, _ := f.s.Create(ctx, f.p, "Item__c", map[string]any{})
	parent, _ := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "P"})
	child, e := f.s.Create(ctx, f.p, "Child__c", map[string]any{"Name": "C", "Parent__c": parent})
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		object string
		id     string
		data   map[string]any
		code   string
		field  string
	}{
		{"Item__c", "", map[string]any{"Small__c": "999.995"}, "NUMBER_OUTSIDE_VALID_RANGE", "Small__c"},
		{"Item__c", "", map[string]any{"Num__c": map[string]any{}}, "JSON_PARSER_ERROR", "Num__c"},
		{"Item__c", "", map[string]any{"Num__c": "1e3"}, "JSON_PARSER_ERROR", "Num__c"},
		{"Item__c", "", map[string]any{"Flag__c": "true"}, "JSON_PARSER_ERROR", "Flag__c"},
		{"Item__c", "", map[string]any{"Day__c": "2026-02-30"}, "JSON_PARSER_ERROR", "Day__c"},
		{"Item__c", "", map[string]any{"At__c": "4001-01-01T00:00:00Z"}, "JSON_PARSER_ERROR", "At__c"},
		{"Item__c", "", map[string]any{"Txt__c": json.Number("5")}, "JSON_PARSER_ERROR", "Txt__c"},
		{"Item__c", "", map[string]any{"Mail__c": "not-an-address"}, "INVALID_EMAIL_ADDRESS", "Mail__c"},
		{"Item__c", "", map[string]any{"Stage__c": "Pending"}, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST", "Stage__c"},
		{"Item__c", "", map[string]any{"Parent__c": "not-an-id"}, "MALFORMED_ID", "Parent__c"},
		{"Item__c", "", map[string]any{"Parent__c": "a00000000000099AAA"}, "INVALID_CROSS_REFERENCE_KEY", ""},
		{"Item__c", "", map[string]any{"Parent__c": otherParent}, "INVALID_CROSS_REFERENCE_KEY", ""},
		{"Item__c", "", map[string]any{"Serial__c": "S-999"}, "INVALID_FIELD_FOR_INSERT_UPDATE", "Serial__c"},
		{"Item__c", "", map[string]any{"Name": "manual"}, "INVALID_FIELD_FOR_INSERT_UPDATE", "Name"},
		{"Contact", "", map[string]any{"LastName": "L", "Name": "Full"}, "INVALID_FIELD_FOR_INSERT_UPDATE", "Name"},
		{"Account", "", map[string]any{"Name": "A", "BillingAddress": map[string]any{"city": "X"}}, "INVALID_FIELD_FOR_INSERT_UPDATE", "BillingAddress"},
		{"Child__c", "", map[string]any{"Name": "orphan"}, "REQUIRED_FIELD_MISSING", "Parent__c"},
		{"Child__c", child, map[string]any{"Parent__c": parent}, "INVALID_FIELD_FOR_INSERT_UPDATE", "Parent__c"},
		{"Item__c", item, map[string]any{"Pct__c": json.Number("1000")}, "NUMBER_OUTSIDE_VALID_RANGE", "Pct__c"},
	}
	for _, c := range cases {
		var err error
		if c.id == "" {
			_, err = f.s.Create(ctx, f.p, c.object, c.data)
		} else {
			err = f.s.Update(ctx, f.p, c.object, c.id, c.data)
		}
		code, fields := apiCode(err)
		if code != c.code || (c.field != "" && (len(fields) != 1 || fields[0] != c.field)) {
			t.Errorf("%s %v: got %v %v (%v), want %s %s", c.object, c.data, code, fields, err, c.code, c.field)
		}
	}
}

func TestRestrictedPicklistCase(t *testing.T) {
	f := newTypedFixture(t)
	_, err := f.s.Create(context.Background(), f.p, "Item__c", map[string]any{"Stage__c": "open"})
	if code, _ := apiCode(err); code != "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST" {
		t.Fatalf("lowercase restricted value: %v", err)
	}
}

func TestDeleteBehaviors(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	p1, _ := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "set-null"})
	p2, _ := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "restrict"})
	item, e := f.s.Create(ctx, f.p, "Item__c", map[string]any{"Parent__c": p1, "Keep__c": p2})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.Delete(ctx, f.p, "Parent__c", p1); e != nil {
		t.Fatal(e)
	}
	if f.stored(t, "Parent__c", item) != "<null>" {
		t.Error("SetNull lookup not cleared")
	}
	if code, _ := apiCode(f.s.Delete(ctx, f.p, "Parent__c", p2)); code != "DELETE_FAILED" {
		t.Errorf("restricted delete code %q", code)
	}
	if f.stored(t, "Keep__c", item) != p2 {
		t.Error("restricted lookup changed")
	}
	master, _ := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "master"})
	child, _ := f.s.Create(ctx, f.p, "Child__c", map[string]any{"Name": "c", "Parent__c": master})
	grand, e := f.s.Create(ctx, f.p, "Grandchild__c", map[string]any{"Name": "g", "Child__c": child})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.s.Delete(ctx, f.p, "Parent__c", master); e != nil {
		t.Fatal(e)
	}
	for object, id := range map[string]string{"Child__c": child, "Grandchild__c": grand} {
		if _, e := f.s.Read(ctx, f.p, object, id); e == nil {
			t.Errorf("%s survived master delete", object)
		}
	}
	account, _ := f.s.Create(ctx, f.p, "Account", map[string]any{"Name": "A"})
	f.s.Create(ctx, f.p, "Contact", map[string]any{"LastName": "L", "AccountId": account})
	if code, _ := apiCode(f.s.Delete(ctx, f.p, "Account", account)); code != "INVALID_CROSS_REFERENCE_KEY" {
		t.Errorf("Contact.AccountId delete behavior changed: %q", code)
	}
}

func TestAutoNumberConcurrent(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make([]string, 8)
	errs := make([]error, 8)
	for i := range ids {
		wg.Add(1)
		go func(ctx context.Context, i int) {
			defer wg.Done()
			ids[i], errs[i] = f.s.Create(ctx, f.p, "Item__c", map[string]any{})
		}(ctx, i)
	}
	wg.Wait()
	seen := map[string]bool{}
	names := []string{}
	for i, id := range ids {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		n := f.stored(t, "Name", id)
		if seen[n] {
			t.Fatalf("duplicate auto-number %s", n)
		}
		seen[n] = true
		names = append(names, n)
	}
	sort.Strings(names)
	if names[0] != "IT-0001" || names[7] != "IT-0008" {
		t.Fatalf("numbers %v", names)
	}
}

func TestHTTPNumberPrecision(t *testing.T) {
	f := newTypedFixture(t)
	h := NewHandler(f.s, "")
	token := "token-" + f.p.Tenant
	body := []byte(`{"Num__c": 1234567890123456.79, "Amount__c": 0.30000000000000004}`)
	r := httptest.NewRequest(http.MethodPost, "/services/data/v65.0/sobjects/Item__c", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	var out struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &out)
	if got := f.stored(t, "Num__c", out.ID); got != "1234567890123456.79" {
		t.Fatalf("Num__c = %q", got)
	}
	if got := f.stored(t, "Amount__c", out.ID); got != "0.30" {
		t.Fatalf("Amount__c = %q", got)
	}
}
