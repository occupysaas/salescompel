package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"salescompel/internal/metadata"
)

// aggInt normalizes the integer widths pgx returns for COUNT and EXTRACT.
func aggInt(t *testing.T, v any) int {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return int(n)
	case int32:
		return int(n)
	case int:
		return n
	case json.Number:
		var i int
		if _, err := fmt.Sscan(string(n), &i); err != nil {
			t.Fatalf("not an integer: %v", v)
		}
		return i
	}
	t.Fatalf("not an integer: %T %v", v, v)
	return 0
}

func aggKeys(rec map[string]any) map[string]bool {
	out := map[string]bool{}
	for k := range rec {
		if k != "attributes" {
			out[k] = true
		}
	}
	return out
}

func TestAggregates(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()

	for _, in := range []map[string]any{
		{"Stage__c": "Closed", "Amount__c": json.Number("0.10"), "Day__c": "2026-03-09", "At__c": "2026-03-09T04:30:00Z"},
		{"Stage__c": "Closed", "Amount__c": json.Number("0.20"), "Day__c": "2026-04-10", "At__c": "2026-04-10T05:00:00Z"},
		{"Stage__c": "Open", "Amount__c": json.Number("0.40"), "Day__c": "2025-12-31", "At__c": "2025-12-31T23:30:00Z"},
	} {
		if _, err := s.Create(ctx, p, "Item__c", in); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("count only envelope", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT COUNT() FROM Item__c")
		if res.TotalSize != 3 || !res.Done {
			t.Fatalf("COUNT() envelope = %#v", res)
		}
		if len(res.Records) != 0 {
			t.Fatalf("COUNT() must not return rows: %#v", res.Records)
		}
	})

	t.Run("grouped aggregate keys and exact decimals", func(t *testing.T) {
		res := soqlQuery(t, s, p,
			"SELECT Stage__c, COUNT(Id) total, SUM(Amount__c), AVG(Amount__c), MIN(Day__c), MAX(At__c) "+
				"FROM Item__c GROUP BY Stage__c HAVING COUNT(Id) > 1 ORDER BY Stage__c")
		if res.TotalSize != 1 || !res.Done || len(res.Records) != 1 {
			t.Fatalf("grouped aggregate = %#v", res)
		}
		rec := res.Records[0]
		attrs, ok := rec["attributes"].(map[string]any)
		if !ok || attrs["type"] != "AggregateResult" {
			t.Fatalf("attributes = %#v", rec["attributes"])
		}
		gotKeys := aggKeys(rec)
		wantKeys := []string{"Stage__c", "total", "expr0", "expr1", "expr2", "expr3"}
		wantSet := map[string]bool{}
		for _, k := range wantKeys {
			wantSet[k] = true
		}
		if len(gotKeys) != len(wantSet) {
			t.Fatalf("aggregate key set = %#v, want exactly %v", gotKeys, wantKeys)
		}
		for _, k := range wantKeys {
			if !gotKeys[k] {
				t.Fatalf("missing aggregate key %q in %#v", k, rec)
			}
		}
		for k := range gotKeys {
			if !wantSet[k] {
				t.Fatalf("unexpected aggregate key %q in %#v", k, rec)
			}
		}
		if rec["Stage__c"] != "Closed" {
			t.Fatalf("Stage__c = %#v", rec["Stage__c"])
		}
		if got := aggInt(t, rec["total"]); got != 2 {
			t.Fatalf("total = %v", rec["total"])
		}
		// Exact decimal encodings: JSON numbers, never float64.
		sum, ok := rec["expr0"].(json.Number)
		if !ok || string(sum) != "0.3" {
			t.Fatalf("SUM(Amount__c) = %T %v, want json.Number(0.3)", rec["expr0"], rec["expr0"])
		}
		avg, ok := rec["expr1"].(json.Number)
		if !ok || string(avg) != "0.15" {
			t.Fatalf("AVG(Amount__c) = %T %v, want json.Number(0.15)", rec["expr1"], rec["expr1"])
		}
		if rec["expr2"] != "2026-03-09" {
			t.Fatalf("MIN(Day__c) = %#v", rec["expr2"])
		}
		if rec["expr3"] != "2026-04-10T05:00:00.000+0000" {
			t.Fatalf("MAX(At__c) = %#v", rec["expr3"])
		}
	})

	t.Run("group by calendar month", func(t *testing.T) {
		res := soqlQuery(t, s, p,
			"SELECT CALENDAR_MONTH(Day__c) m, COUNT(Id) FROM Item__c "+
				"GROUP BY CALENDAR_MONTH(Day__c) ORDER BY m")
		if res.TotalSize != 3 || len(res.Records) != 3 {
			t.Fatalf("month groups = %#v", res)
		}
		wantMonths := []int{3, 4, 12}
		for i, rec := range res.Records {
			if got := aggInt(t, rec["m"]); got != wantMonths[i] {
				t.Fatalf("month[%d] = %v want %d", i, rec["m"], wantMonths[i])
			}
			if got := aggInt(t, rec["expr0"]); got != 1 {
				t.Fatalf("month count[%d] = %v", i, rec["expr0"])
			}
		}
	})

	t.Run("count distinct", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT COUNT_DISTINCT(Stage__c) FROM Item__c")
		if res.TotalSize != 1 || len(res.Records) != 1 {
			t.Fatalf("count distinct = %#v", res)
		}
		if got := aggInt(t, res.Records[0]["expr0"]); got != 2 {
			t.Fatalf("COUNT_DISTINCT = %v", res.Records[0]["expr0"])
		}
	})

	t.Run("min max numeric exact text encoding", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT MIN(Amount__c), MAX(Amount__c) FROM Item__c")
		if res.TotalSize != 1 || len(res.Records) != 1 {
			t.Fatalf("min/max = %#v", res)
		}
		rec := res.Records[0]
		min, ok := rec["expr0"].(json.Number)
		if !ok || string(min) != "0.1" {
			t.Fatalf("MIN(Amount__c) = %T %v, want json.Number(0.1)", rec["expr0"], rec["expr0"])
		}
		max, ok := rec["expr1"].(json.Number)
		if !ok || string(max) != "0.4" {
			t.Fatalf("MAX(Amount__c) = %T %v, want json.Number(0.4)", rec["expr1"], rec["expr1"])
		}
	})

	t.Run("invalid aggregate shapes refused", func(t *testing.T) {
		for _, query := range []string{
			"SELECT COUNT(Id), (SELECT Id FROM Contacts) FROM Account",
			"SELECT Name, (SELECT Id FROM Contacts) FROM Account GROUP BY Name",
			"SELECT COUNT(Id) FROM Item__c GROUP BY COUNT(Id)",
			"SELECT COUNT() FROM Item__c GROUP BY Stage__c",
			"SELECT toLabel(Stage__c), COUNT(Id) FROM Item__c",
			"SELECT DAY_ONLY(At__c), COUNT(Id) FROM Item__c",
			"SELECT Name, COUNT(Id) FROM Item__c GROUP BY Stage__c",
			"SELECT Stage__c, COUNT(Id) FROM Item__c GROUP BY toLabel(Stage__c)",
			// Ungrouped aggregate ORDER BY/HAVING may reference aggregates only.
			"SELECT COUNT() FROM Item__c ORDER BY Txt__c",
			"SELECT COUNT(Id) FROM Item__c HAVING Txt__c = 'x'",
			// MIN/MAX kind allowlist: these never reach PostgreSQL.
			"SELECT MIN(Flag__c) FROM Item__c",
			"SELECT MAX(Id) FROM Item__c",
		} {
			err := soqlErr(t, s, p, query)
			if code, _ := apiCode(err); code != "MALFORMED_QUERY" {
				t.Fatalf("%s: got %v, want MALFORMED_QUERY", query, err)
			}
		}
	})

	t.Run("ungrouped aggregate clauses", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT COUNT(Id) FROM Item__c HAVING COUNT(Id) > 1 ORDER BY COUNT(Id)")
		if res.TotalSize != 1 || len(res.Records) != 1 {
			t.Fatalf("ungrouped aggregate clause result = %#v", res)
		}
		if got := aggInt(t, res.Records[0]["expr0"]); got != 3 {
			t.Fatalf("COUNT(Id) = %v want 3", res.Records[0]["expr0"])
		}
	})

	t.Run("grouped parent path nulls", func(t *testing.T) {
		gF := newTypedFixture(t)
		parent, err := gF.s.Create(ctx, gF.p, "Parent__c", map[string]any{"Name": "P"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = gF.s.Create(ctx, gF.p, "Item__c", map[string]any{"Txt__c": "with", "Parent__c": parent}); err != nil {
			t.Fatal(err)
		}
		if _, err = gF.s.Create(ctx, gF.p, "Item__c", map[string]any{"Txt__c": "without"}); err != nil {
			t.Fatal(err)
		}
		res := soqlQuery(t, gF.s, gF.p,
			"SELECT Parent__r.Name, COUNT(Id) FROM Item__c GROUP BY Parent__r.Name ORDER BY Parent__r.Name")
		if res.TotalSize != 2 || len(res.Records) != 2 {
			t.Fatalf("grouped parent path = %#v", res)
		}
		nullSeen, namedSeen := false, false
		for _, rec := range res.Records {
			if got := aggInt(t, rec["expr0"]); got != 1 {
				t.Fatalf("group count = %v", rec["expr0"])
			}
			if rec["Parent__r"] == nil {
				nullSeen = true
				continue
			}
			rel, ok := rec["Parent__r"].(map[string]any)
			if !ok || rel["Name"] != "P" {
				t.Fatalf("grouped parent path record = %#v", rec)
			}
			namedSeen = true
		}
		if !nullSeen || !namedSeen {
			t.Fatalf("null=%v named=%v records=%#v", nullSeen, namedSeen, res.Records)
		}
	})

	t.Run("more than 2000 groups", func(t *testing.T) {
		capF := newTypedFixture(t)
		txtCol := capF.cols["Item__c.Txt__c"]
		serialCol := capF.cols["Item__c.Serial__c"]
		_, err := capF.s.pool.Exec(ctx,
			`INSERT INTO `+capF.items.TableSQL()+` (tenant_id,id,name,"`+txtCol+`","`+serialCol+`")
			 SELECT $1,$2||lpad(g::text,15,'0'),'Bulk-'||g,'G'||g,'BulkS-'||g FROM generate_series(1,2001) g`,
			capF.p.Tenant, capF.items.Prefix)
		if err != nil {
			t.Fatal(err)
		}
		err = soqlErr(t, capF.s, capF.p, "SELECT Txt__c, COUNT(Id) FROM Item__c GROUP BY Txt__c")
		if code, _ := apiCode(err); code != "QUERY_TOO_COMPLICATED" {
			t.Fatalf("group cap error = %v", err)
		}
	})
}

func TestToLabelAndFields(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()

	if _, err := metadata.CreateField(ctx, s.pool, p.Tenant, metadata.FieldInput{
		Object: "Item__c", Name: "Status__c", Label: "Status", Type: "Picklist",
		Values: []metadata.PicklistValue{
			{Active: true, Value: "Closed", Label: "Closed label"},
			{Active: true, Value: "Open", Label: "O'Brien"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"Closed", "Open"} {
		data := map[string]any{"Status__c": status}
		if status == "Closed" {
			data["Multi__c"] = "A;B"
		}
		if _, err := s.Create(ctx, p, "Item__c", data); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("toLabel select returns labels", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT toLabel(Status__c) FROM Item__c ORDER BY Name")
		if res.TotalSize != 2 {
			t.Fatalf("toLabel result = %#v", res)
		}
		got := map[string]bool{}
		for _, rec := range res.Records {
			label, ok := rec["expr0"].(string)
			if !ok {
				t.Fatalf("toLabel value = %T %v", rec["expr0"], rec["expr0"])
			}
			got[label] = true
		}
		if !got["Closed label"] || !got["O'Brien"] {
			t.Fatalf("labels = %#v", got)
		}
	})

	t.Run("toLabel label with apostrophe", func(t *testing.T) {
		res := soqlQuery(t, s, p, `SELECT toLabel(Status__c) FROM Item__c WHERE toLabel(Status__c) = 'O\'Brien'`)
		if res.TotalSize != 1 || res.Records[0]["expr0"] != "O'Brien" {
			t.Fatalf("apostrophe label = %#v", res)
		}
	})

	t.Run("toLabel multiselect elements", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT toLabel(Multi__c) FROM Item__c WHERE Multi__c INCLUDES ('A')")
		if res.TotalSize != 1 || res.Records[0]["expr0"] != "A;B" {
			t.Fatalf("multiselect toLabel = %#v", res)
		}
	})

	t.Run("toLabel filter matches label not value", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE toLabel(Status__c) = 'Closed label'")
		if res.TotalSize != 1 {
			t.Fatalf("label filter = %d", res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE toLabel(Status__c) = 'Closed'")
		if res.TotalSize != 0 {
			t.Fatalf("toLabel filtered by API value, want label semantics: %d", res.TotalSize)
		}
	})

	t.Run("toLabel order by refused", func(t *testing.T) {
		for _, q := range []string{
			"SELECT Id FROM Item__c ORDER BY toLabel(Status__c)",
			"SELECT toLabel(Status__c) s FROM Item__c ORDER BY s",
		} {
			err := soqlErr(t, s, p, q)
			if code, _ := apiCode(err); code != "MALFORMED_QUERY" {
				t.Fatalf("%s: got %v, want MALFORMED_QUERY", q, err)
			}
		}
	})

	t.Run("fields standard on custom object", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT FIELDS(STANDARD) FROM Item__c")
		if res.TotalSize != 2 {
			t.Fatalf("FIELDS(STANDARD) = %#v", res)
		}
		for _, rec := range res.Records {
			keys := aggKeys(rec)
			if len(keys) != 2 || !keys["Id"] || !keys["Name"] {
				t.Fatalf("FIELDS(STANDARD) keys = %#v", keys)
			}
		}
	})

	t.Run("fields all describe order and policy", func(t *testing.T) {
		if _, err := s.Create(ctx, p, "Account", map[string]any{"Name": "Fields", "Compat_Text__c": "secret"}); err != nil {
			t.Fatal(err)
		}
		snap := metadata.Snapshot{Objects: metadata.StandardObjects()}
		account, err := snap.Object("Account")
		if err != nil {
			t.Fatal(err)
		}
		restricted := Principal{Tenant: p.Tenant, User: "restricted", Role: "restricted"}

		for _, tc := range []struct {
			name       string
			principal  Principal
			omitDenied bool
		}{
			{name: "admin", principal: p},
			{name: "restricted", principal: restricted, omitDenied: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				res, md, err := s.QueryColumns(ctx, tc.principal, "SELECT FIELDS(ALL) FROM Account LIMIT 200", 2000)
				if err != nil {
					t.Fatal(err)
				}
				if res.TotalSize != 1 || len(res.Records) != 1 {
					t.Fatalf("FIELDS(ALL) result = %#v", res)
				}
				var want []string
				for _, field := range account.Fields {
					if tc.omitDenied && field.Name == "Compat_Text__c" {
						continue
					}
					want = append(want, field.Name)
				}
				if len(md) != len(want) {
					t.Fatalf("column metadata count = %d want %d: %#v", len(md), len(want), md)
				}
				for i, name := range want {
					if md[i]["columnName"] != name {
						t.Fatalf("column metadata[%d] = %v want %s (describe order)", i, md[i]["columnName"], name)
					}
				}
				_, hasDenied := res.Records[0]["Compat_Text__c"]
				if tc.omitDenied && hasDenied {
					t.Fatal("restricted FIELDS(ALL) leaked a denied field")
				}
				if !tc.omitDenied && !hasDenied {
					t.Fatal("admin FIELDS(ALL) dropped a readable field")
				}
			})
		}
	})
}

func TestDateFunctions(t *testing.T) {
	f := newTypedFixture(t)
	s, p := f.s, f.p
	ctx := context.Background()
	for _, in := range []map[string]any{
		{"Txt__c": "UTC", "At__c": "2026-03-09T04:30:00Z", "Day__c": "2026-03-09"},
		{"Txt__c": "Offset", "At__c": "2026-03-09T23:30:00-05:00", "Day__c": "2026-03-10"},
	} {
		if _, err := s.Create(ctx, p, "Item__c", in); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("calendar year", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE CALENDAR_YEAR(At__c) = 2026")
		if res.TotalSize != 2 {
			t.Fatalf("CALENDAR_YEAR = %d", res.TotalSize)
		}
		res = soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE CALENDAR_YEAR(Day__c) = 2026")
		if res.TotalSize != 2 {
			t.Fatalf("date-field CALENDAR_YEAR = %d", res.TotalSize)
		}
	})

	t.Run("day only in utc", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Txt__c FROM Item__c WHERE DAY_ONLY(At__c) = 2026-03-09")
		if res.TotalSize != 1 || res.Records[0]["Txt__c"] != "UTC" {
			t.Fatalf("DAY_ONLY = %#v", res)
		}
		res = soqlQuery(t, s, p, "SELECT Txt__c FROM Item__c WHERE DAY_ONLY(At__c) = 2026-03-10")
		if res.TotalSize != 1 || res.Records[0]["Txt__c"] != "Offset" {
			t.Fatalf("DAY_ONLY offset = %#v", res)
		}
	})

	t.Run("hour in day in utc", func(t *testing.T) {
		res := soqlQuery(t, s, p, "SELECT Id FROM Item__c WHERE HOUR_IN_DAY(At__c) = 4")
		if res.TotalSize != 2 {
			t.Fatalf("HOUR_IN_DAY = %d", res.TotalSize)
		}
	})
}
