package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"salescompel/internal/metadata"
	"strings"
	"testing"
)

func describeMap(t *testing.T, f typedFixture, object string) map[string]map[string]any {
	t.Helper()
	d, e := f.s.Describe(context.Background(), f.p, object)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(d)
	var out struct{ Fields []map[string]any }
	json.Unmarshal(raw, &out)
	m := map[string]map[string]any{}
	for _, field := range out.Fields {
		m[field["name"].(string)] = field
	}
	return m
}

func TestDescribeEveryKind(t *testing.T) {
	f := newTypedFixture(t)
	golden := map[string]any{}
	for _, object := range []string{"Item__c", "Child__c", "Account", "Contact"} {
		golden[object] = describeMap(t, f, object)
	}
	item, child, account, contact := golden["Item__c"].(map[string]map[string]any), golden["Child__c"].(map[string]map[string]any), golden["Account"].(map[string]map[string]any), golden["Contact"].(map[string]map[string]any)
	checks := []struct {
		field map[string]any
		key   string
		want  any
	}{
		{item["Name"], "autoNumber", true}, {item["Name"], "createable", false}, {item["Name"], "nameField", true},
		{item["Serial__c"], "updateable", false}, {item["Flag__c"], "nillable", false}, {item["Flag__c"], "defaultValue", true},
		{item["Flag__c"], "type", "boolean"}, {item["Stage__c"], "restrictedPicklist", true}, {item["Num__c"], "precision", float64(18)},
		{item["Num__c"], "scale", float64(2)}, {item["Num__c"], "type", "double"}, {item["Amount__c"], "type", "currency"},
		{item["At__c"], "soapType", "xsd:dateTime"}, {item["Long__c"], "filterable", false}, {item["Multi__c"], "sortable", false},
		{item["Parent__c"], "relationshipName", "Parent__r"}, {item["Keep__c"], "restrictedDelete", true},
		{child["Parent__c"], "updateable", false}, {child["Parent__c"], "nillable", false}, {child["Parent__c"], "cascadeDelete", true},
		{child["Parent__c"], "relationshipOrder", float64(0)}, {account["BillingAddress"], "type", "address"},
		{account["BillingAddress"], "createable", false}, {account["BillingCity"], "compoundFieldName", "BillingAddress"},
		{account["NumberOfEmployees"], "type", "int"}, {account["NumberOfEmployees"], "digits", float64(8)},
		{contact["Name"], "createable", false}, {contact["Name"], "updateable", false}, {contact["AccountId"], "relationshipName", "Account"},
		{contact["Email"], "type", "email"}, {contact["Id"], "createable", false},
	}
	for i, c := range checks {
		if c.field == nil || c.field[c.key] != c.want {
			t.Errorf("check %d: %s = %v want %v", i, c.key, c.field[c.key], c.want)
		}
	}
	raw, _ := json.MarshalIndent(golden, "", " ")
	path := "testdata/describe-field-types.json"
	if os.Getenv("UPDATE_GOLDEN") != "" {
		os.WriteFile(path, append(raw, '\n'), 0o644)
	}
	want, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(want)) != strings.TrimSpace(string(raw)) {
		t.Fatalf("describe differs from %s (%v); rerun with UPDATE_GOLDEN=1 and review", path, err)
	}
}

func soql(t *testing.T, f typedFixture, soql string) ([]map[string]any, error) {
	t.Helper()
	q, e := ParseQuery(soql)
	if e != nil {
		return nil, e
	}
	return f.s.Select(context.Background(), f.p, q)
}

func TestQueryTypedProjection(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	parent, _ := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "P"})
	id, e := f.s.Create(ctx, f.p, "Item__c", map[string]any{"Num__c": json.Number("0.30000000000000004"), "Amount__c": "12.50", "Pct__c": json.Number("7"),
		"Day__c": "2026-11-11", "At__c": "2026-03-08T23:30:00-05:00", "Multi__c": "C;A", "Mail__c": "a@b.co", "Stage__c": "Closed", "Parent__c": parent})
	if e != nil {
		t.Fatal(e)
	}
	f.s.Create(ctx, f.p, "Item__c", map[string]any{"Num__c": json.Number("-5"), "Flag__c": false})
	rows, e := soql(t, f, "SELECT Id,Name,Num__c,Amount__c,Pct__c,Day__c,At__c,Flag__c,Multi__c,Stage__c,Parent__c,Serial__c FROM Item__c ORDER BY Num__c DESC LIMIT 10")
	if e != nil || len(rows) != 2 {
		t.Fatal(rows, e)
	}
	r := rows[0]
	want := map[string]any{"Name": "IT-0001", "Num__c": json.Number("0.3"), "Amount__c": json.Number("12.5"), "Pct__c": json.Number("7"),
		"Day__c": "2026-11-11", "At__c": "2026-03-09T04:30:00.000+0000", "Flag__c": true, "Multi__c": "A;C", "Stage__c": "Closed", "Parent__c": parent, "Serial__c": "S-007", "Id": id}
	for k, w := range want {
		if r[k] != w {
			t.Errorf("%s = %#v want %#v", k, r[k], w)
		}
	}
	if rows[1]["Num__c"] != json.Number("-5") || rows[1]["Flag__c"] != false || rows[1]["Day__c"] != nil {
		t.Errorf("second row %#v", rows[1])
	}
	for _, clause := range []string{"ORDER BY Day__c", "ORDER BY At__c", "ORDER BY Flag__c", "ORDER BY Stage__c", "WHERE Stage__c = 'Closed'", "WHERE Mail__c = 'A@B.CO'", "WHERE Parent__c = '" + parent[:15] + "'"} {
		got, e := soql(t, f, "SELECT Id FROM Item__c "+clause+" LIMIT 10")
		if e != nil || len(got) == 0 {
			t.Errorf("%s: %v %v", clause, got, e)
		}
	}
	for _, clause := range []string{"WHERE Num__c = '0.3'", "WHERE Day__c = '2026-11-11'", "WHERE At__c = 'x'", "WHERE Flag__c = 'true'"} {
		_, e := soql(t, f, "SELECT Id FROM Item__c "+clause+" LIMIT 10")
		if code, _ := apiCode(e); code != "MALFORMED_QUERY" || !strings.Contains(e.Error(), "not yet supported") {
			t.Errorf("%s: %v", clause, e)
		}
	}
	for _, clause := range []string{"WHERE Long__c = 'x'", "ORDER BY Long__c", "ORDER BY Multi__c", "WHERE Multi__c = 'A'"} {
		_, e := soql(t, f, "SELECT Id FROM Item__c "+clause+" LIMIT 10")
		if code, _ := apiCode(e); code != "INVALID_FIELD" {
			t.Errorf("%s: %v", clause, e)
		}
	}
	read, e := f.s.Read(ctx, f.p, "Item__c", id)
	if e != nil || read["Amount__c"] != json.Number("12.5") || read["At__c"] != "2026-03-09T04:30:00.000+0000" {
		t.Fatalf("Read %v %#v", e, read)
	}
}

func TestAddressRead(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	partial, _ := f.s.Create(ctx, f.p, "Account", map[string]any{"Name": "A", "BillingCity": "Lyon", "BillingCountry": "France"})
	empty, _ := f.s.Create(ctx, f.p, "Account", map[string]any{"Name": "B"})
	row, e := f.s.Read(ctx, f.p, "Account", partial)
	if e != nil {
		t.Fatal(e)
	}
	addr, ok := row["BillingAddress"].(map[string]any)
	if !ok || addr["city"] != "Lyon" || addr["country"] != "France" || addr["street"] != nil || addr["state"] != nil || addr["postalCode"] != nil || len(addr) != 5 {
		t.Fatalf("BillingAddress %#v", row["BillingAddress"])
	}
	if row["ShippingAddress"] != nil {
		t.Fatalf("empty ShippingAddress %#v", row["ShippingAddress"])
	}
	row, _ = f.s.Read(ctx, f.p, "Account", empty)
	if row["BillingAddress"] != nil {
		t.Fatalf("empty BillingAddress %#v", row["BillingAddress"])
	}
	rows, e := soql(t, f, "SELECT Id,BillingAddress FROM Account WHERE Id = '"+partial+"'")
	if e != nil || rows[0]["BillingAddress"].(map[string]any)["city"] != "Lyon" {
		t.Fatalf("query address %v %#v", e, rows)
	}
	contact, _ := f.s.Create(ctx, f.p, "Contact", map[string]any{"FirstName": "Ada", "LastName": "Lovelace", "Birthdate": "1815-12-10"})
	row, _ = f.s.Read(ctx, f.p, "Contact", contact)
	if row["Name"] != "Ada Lovelace" || row["Birthdate"] != "1815-12-10" {
		t.Fatalf("contact %#v", row)
	}
}

func TestCompositeCascadeRollback(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	master, _ := f.s.Create(ctx, f.p, "Parent__c", map[string]any{"Name": "M"})
	child, _ := f.s.Create(ctx, f.p, "Child__c", map[string]any{"Name": "C", "Parent__c": master})
	grand, e := f.s.Create(ctx, f.p, "Grandchild__c", map[string]any{"Name": "G", "Child__c": child})
	if e != nil {
		t.Fatal(e)
	}
	f.s.Composite(ctx, f.p, compReq(compSub("DELETE", "Parent__c", master, "d", nil), compSub("POST", "Item__c", "", "i", map[string]any{"Stage__c": "Open"}), compSub("POST", "Item__c", "", "bad", map[string]any{"Stage__c": "Nope"})))
	for object, id := range map[string]string{"Parent__c": master, "Child__c": child, "Grandchild__c": grand} {
		if _, e := f.s.Read(ctx, f.p, object, id); e != nil {
			t.Errorf("%s not restored: %v", object, e)
		}
	}
	var n int
	f.s.pool.QueryRow(ctx, "SELECT count(*) FROM "+f.items.TableSQL()).Scan(&n)
	if n != 0 {
		t.Fatalf("rolled-back item visible: %d", n)
	}
	id, _ := f.s.Create(ctx, f.p, "Item__c", map[string]any{})
	if name := f.stored(t, "Name", id); name == "IT-0001" {
		t.Log("counter reused after rollback (allowed: gaps are optional)")
	}
}

func TestCurrencyBridge(t *testing.T) {
	f := newTypedFixture(t)
	f.s.SetCurrency(context.Background(), f.p.Tenant, "EUR")
	h := NewHandler(f.s, "")
	get := func(q string) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodGet, "/services/data/v65.0/query?q="+url.QueryEscape(q), nil)
		r.Header.Set("Authorization", "Bearer token-"+f.p.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	code, out := get("SELECT  DefaultCurrencyIsoCode   FROM Organization")
	records, _ := out["records"].([]any)
	if code != 200 || len(records) != 1 || records[0].(map[string]any)["DefaultCurrencyIsoCode"] != "EUR" {
		t.Fatalf("bridge %d %v", code, out)
	}
	if code, _ := get("SELECT DefaultCurrencyIsoCode FROM Organizations"); code != 400 {
		t.Fatalf("near miss served %d", code)
	}
}

func TestStoredFieldsEndpoint(t *testing.T) {
	f := newTypedFixture(t)
	ctx := context.Background()
	if _, e := metadata.CreateStoredField(ctx, f.s.pool, f.p.Tenant, "Item__c", []byte(`{"name":"Score__c","label":"Score","type":"Formula","formula":"1"}`)); e != nil {
		t.Fatal(e)
	}
	h := NewHandler(f.s, "")
	get := func(path, token string) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodGet, "/services/data/v65.0/"+path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	code, out := get("candidate/sobjects/item__c/storedFields", "token-"+f.p.Tenant)
	list, _ := out["storedFields"].([]any)
	if code != 200 || len(list) != 1 || list[0].(map[string]any)["name"] != "Score__c" || list[0].(map[string]any)["type"] != "Formula" || list[0].(map[string]any)["label"] != "Score" {
		t.Fatalf("stored fields %d %v", code, out)
	}
	if code, out = get("candidate/sobjects/Parent__c/storedFields", "token-"+f.p.Tenant); code != 200 || len(out["storedFields"].([]any)) != 0 {
		t.Fatalf("empty list %d %v", code, out)
	}
	if code, _ = get("candidate/sobjects/Item__c/storedFields", "token-"+f.other.Tenant); code != 404 {
		t.Fatalf("other tenant saw object: %d", code)
	}
	d, _ := f.s.Describe(ctx, f.p, "Item__c")
	for _, fd := range d.Fields {
		if fd.Name == "Score__c" {
			t.Fatal("stored field described")
		}
	}
}
