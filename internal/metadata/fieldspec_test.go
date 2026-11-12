package metadata_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"salescompel/internal/fieldtype"
	. "salescompel/internal/metadata"
	"salescompel/internal/records"
	"strings"
	"testing"
)

func specSnapshot() (Snapshot, Object) {
	s := Snapshot{Tenant: "t", Objects: StandardObjects()}
	project := Object{Name: "Project__c", Label: "Project", PluralLabel: "Projects", Schema: "sc_records", Table: "o_x", Fields: []Field{{Name: "Id", Options: Options{Kind: fieldtype.Id}}}}
	task := Object{Name: "Task__c", Label: "Task", PluralLabel: "Tasks", Schema: "sc_records", Table: "o_y", Fields: []Field{{Name: "Id", Options: Options{Kind: fieldtype.Id}}}}
	s.Objects = append(s.Objects, project, task)
	return s, project
}

func values(n int) []PicklistValue {
	out := make([]PicklistValue, n)
	for i := range out {
		out[i] = PicklistValue{Value: fmt.Sprintf("V%d", i), Label: fmt.Sprintf("V%d", i), Active: true}
	}
	return out
}

func TestBuildFieldRules(t *testing.T) {
	s, project := specSnapshot()
	account, _ := s.Object("Account")
	ok := []FieldInput{
		{Name: "A__c", Label: "A", Type: "Text", Length: 80},
		{Name: "A__c", Label: "A", Type: "TextArea"},
		{Name: "A__c", Label: "A", Type: "LongTextArea", Length: 131072, VisibleLines: 5},
		{Name: "A__c", Label: "A", Type: "Number", Precision: 18, Scale: 18},
		{Name: "A__c", Label: "A", Type: "Currency"},
		{Name: "A__c", Label: "A", Type: "Percent", Precision: 5, Scale: 2},
		{Name: "A__c", Label: "A", Type: "Date"},
		{Name: "A__c", Label: "A", Type: "DateTime"},
		{Name: "A__c", Label: "A", Type: "Checkbox", Default: ptr("true")},
		{Name: "A__c", Label: "A", Type: "Picklist", Values: values(1000), Restricted: true, Default: ptr("V1")},
		{Name: "A__c", Label: "A", Type: "MultiselectPicklist", Values: values(3), VisibleLines: 4},
		{Name: "A__c", Label: "A", Type: "Email"},
		{Name: "A__c", Label: "A", Type: "Phone"},
		{Name: "A__c", Label: "A", Type: "Url"},
		{Name: "A__c", Label: "A", Type: "Lookup", ReferenceTo: "Project__c"},
		{Name: "A__c", Label: "A", Type: "Lookup", ReferenceTo: "account", DeleteBehavior: "Restrict"},
		{Name: "A__c", Label: "A", Type: "MasterDetail", ReferenceTo: "Task__c", Reparentable: true},
		{Name: "A__c", Label: "A", Type: "AutoNumber", DisplayFormat: "A-{0000}", StartingNumber: 1},
		{Name: "A__c", Label: "A", Type: "Text", Length: 10, Required: true},
	}
	for _, in := range ok {
		f, ddl, err := BuildField(in, project, s, false)
		if err != nil {
			t.Errorf("%s rejected: %v", in.Type, err)
			continue
		}
		if f.Column == "" || ddl.Type == "" || f.Options.Kind != fieldtype.Kind(in.Type) {
			t.Errorf("%s: incomplete %#v %#v", in.Type, f, ddl)
		}
	}
	bad := map[string]FieldInput{
		"precision 0":               {Name: "A__c", Label: "A", Type: "Number", Precision: 0, Scale: 1},
		"precision 19":              {Name: "A__c", Label: "A", Type: "Number", Precision: 19},
		"scale > precision":         {Name: "A__c", Label: "A", Type: "Number", Precision: 4, Scale: 5},
		"long text 255":             {Name: "A__c", Label: "A", Type: "LongTextArea", Length: 255},
		"long text 131073":          {Name: "A__c", Label: "A", Type: "LongTextArea", Length: 131073},
		"no values":                 {Name: "A__c", Label: "A", Type: "Picklist"},
		"1001 values":               {Name: "A__c", Label: "A", Type: "Picklist", Values: values(1001)},
		"256-char value":            {Name: "A__c", Label: "A", Type: "Picklist", Values: []PicklistValue{{Value: strings.Repeat("x", 256), Label: "x", Active: true}}},
		"duplicate values":          {Name: "A__c", Label: "A", Type: "Picklist", Values: []PicklistValue{{Value: "A", Label: "A", Active: true}, {Value: "a", Label: "a", Active: true}}},
		"default not listed":        {Name: "A__c", Label: "A", Type: "Picklist", Values: values(2), Default: ptr("Z")},
		"semicolon in multi":        {Name: "A__c", Label: "A", Type: "MultiselectPicklist", Values: []PicklistValue{{Value: "A;B", Label: "A", Active: true}}},
		"unknown lookup":            {Name: "A__c", Label: "A", Type: "Lookup", ReferenceTo: "Nope__c"},
		"bad delete behavior":       {Name: "A__c", Label: "A", Type: "Lookup", ReferenceTo: "Account", DeleteBehavior: "Cascade"},
		"master self":               {Name: "A__c", Label: "A", Type: "MasterDetail", ReferenceTo: "Project__c"},
		"master with delete option": {Name: "A__c", Label: "A", Type: "MasterDetail", ReferenceTo: "Task__c", DeleteBehavior: "SetNull"},
		"autonumber format":         {Name: "A__c", Label: "A", Type: "AutoNumber", DisplayFormat: "NONE"},
		"precision on email":        {Name: "A__c", Label: "A", Type: "Email", Precision: 3},
		"length on date":            {Name: "A__c", Label: "A", Type: "Date", Length: 3},
		"values on text":            {Name: "A__c", Label: "A", Type: "Text", Length: 5, Values: values(1)},
		"required checkbox":         {Name: "A__c", Label: "A", Type: "Checkbox", Required: true},
		"required autonumber":       {Name: "A__c", Label: "A", Type: "AutoNumber", DisplayFormat: "A-{0}", Required: true},
		"required master":           {Name: "A__c", Label: "A", Type: "MasterDetail", ReferenceTo: "Task__c", Required: true},
		"checkbox default":          {Name: "A__c", Label: "A", Type: "Checkbox", Default: ptr("yes")},
		"default on number":         {Name: "A__c", Label: "A", Type: "Number", Precision: 3, Default: ptr("1")},
		"unknown type":              {Name: "A__c", Label: "A", Type: "Time"},
		"stored type":               {Name: "A__c", Label: "A", Type: "Formula"},
		"bad name":                  {Name: "A", Label: "A", Type: "Date"},
		"bad relationship name":     {Name: "A__c", Label: "A", Type: "Lookup", ReferenceTo: "Account", RelationshipName: "bad name"},
	}
	for name, in := range bad {
		if _, _, err := BuildField(in, project, s, false); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, _, err := BuildField(FieldInput{Name: "A__c", Label: "A", Type: "MasterDetail", ReferenceTo: "Task__c"}, account, s, false); err == nil {
		t.Error("master-detail with a standard child accepted")
	}
	if _, _, err := BuildField(FieldInput{Name: "A__c", Label: "A", Type: "Text", Length: 5, Required: true}, project, s, true); err == nil {
		t.Error("required field on a non-empty object accepted")
	}
	if _, _, err := BuildField(FieldInput{Name: "A__c", Label: "A", Type: "MasterDetail", ReferenceTo: "Task__c"}, project, s, true); err == nil {
		t.Error("master-detail on a non-empty object accepted")
	}
	two := project
	for i, name := range []string{"M1__c", "M2__c"} {
		f, _, err := BuildField(FieldInput{Name: name, Label: name, Type: "MasterDetail", ReferenceTo: "Task__c"}, two, s, false)
		if err != nil || f.RelationshipOrder == nil || *f.RelationshipOrder != i {
			t.Fatalf("master %d: %v %#v", i, err, f)
		}
		two.Fields = append(two.Fields, f)
	}
	if _, _, err := BuildField(FieldInput{Name: "M3__c", Label: "M3", Type: "MasterDetail", ReferenceTo: "Account"}, two, s, false); err == nil {
		t.Error("third master-detail accepted")
	}
	f, _, _ := BuildField(FieldInput{Name: "Owner_Project__c", Label: "P", Type: "Lookup", ReferenceTo: "Project__c"}, project, s, false)
	if f.RelationshipName == nil || *f.RelationshipName != "Owner_Project__r" || f.ReferenceTo[0] != "Project__c" || !f.Nillable {
		t.Errorf("lookup defaults %#v", f)
	}
}

func ptr[T any](v T) *T { return &v }

func TestCreateFieldDDL(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "ddl")
	o, e := CreateObject(ctx, p, "ddl", ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]string{
		"T__c": "character varying|80|YES|", "TA__c": "character varying|255|YES|", "L__c": "text||YES|",
		"N__c": "numeric||YES|", "D__c": "date||YES|", "DT__c": "timestamp with time zone||YES|",
		"C__c": "boolean||NO|true", "P__c": "character varying|255|YES|", "M__c": "character varying|4099|YES|",
		"E__c": "character varying|80|YES|", "PH__c": "character varying|40|YES|", "U__c": "character varying|255|YES|",
		"LK__c": "character varying|18|YES|", "AN__c": "character varying|30|NO|",
	}
	inputs := []FieldInput{
		{Name: "T__c", Type: "Text", Length: 80}, {Name: "TA__c", Type: "TextArea"}, {Name: "L__c", Type: "LongTextArea", Length: 1000},
		{Name: "N__c", Type: "Number", Precision: 10, Scale: 2}, {Name: "D__c", Type: "Date"}, {Name: "DT__c", Type: "DateTime"},
		{Name: "C__c", Type: "Checkbox", Default: ptr("true")}, {Name: "P__c", Type: "Picklist", Values: values(2)}, {Name: "M__c", Type: "MultiselectPicklist", Values: values(2)},
		{Name: "E__c", Type: "Email"}, {Name: "PH__c", Type: "Phone"}, {Name: "U__c", Type: "Url"},
		{Name: "LK__c", Type: "Lookup", ReferenceTo: "Account"}, {Name: "AN__c", Type: "AutoNumber", DisplayFormat: "A-{000}", StartingNumber: 1},
	}
	columns := map[string]string{}
	for _, in := range inputs {
		in.Object, in.Label = "Project__c", in.Name
		f, e := CreateField(ctx, p, "ddl", in)
		if e != nil {
			t.Fatalf("%s: %v", in.Name, e)
		}
		columns[in.Name] = f.Column
	}
	for name, w := range want {
		var got string
		e := p.QueryRow(ctx, `SELECT data_type||'|'||coalesce(character_maximum_length::text,'')||'|'||is_nullable||'|'||coalesce(column_default,'') FROM information_schema.columns WHERE table_schema='sc_records' AND table_name=$1 AND column_name=$2`, o.Table, columns[name]).Scan(&got)
		if e != nil || got != w {
			t.Errorf("%s column %q %v want %q", name, got, e, w)
		}
	}
	var precision, scale int
	p.QueryRow(ctx, `SELECT numeric_precision,numeric_scale FROM information_schema.columns WHERE table_name=$1 AND column_name=$2`, o.Table, columns["N__c"]).Scan(&precision, &scale)
	if precision != 10 || scale != 2 {
		t.Errorf("numeric(%d,%d)", precision, scale)
	}
	other, _ := CreateObject(ctx, p, "ddl", ObjectInput{Name: "Task__c", Label: "Task", PluralLabel: "Tasks"})
	for _, in := range []FieldInput{
		{Name: "SN__c", Type: "Lookup", ReferenceTo: "Project__c", ChildRelationshipName: "SetNulls__r"},
		{Name: "RS__c", Type: "Lookup", ReferenceTo: "Project__c", DeleteBehavior: "Restrict", ChildRelationshipName: "Restricts__r"},
		{Name: "MD__c", Type: "MasterDetail", ReferenceTo: "Project__c", ChildRelationshipName: "Cascades__r"},
	} {
		in.Object, in.Label = "Task__c", in.Name
		f, e := CreateField(ctx, p, "ddl", in)
		if e != nil {
			t.Fatalf("%s: %v", in.Name, e)
		}
		var rule string
		e = p.QueryRow(ctx, `SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname=$1`, "fk_"+f.ID).Scan(&rule)
		wantRule := map[string]string{"SN__c": "ON DELETE SET NULL (" + f.Column + ")", "RS__c": "ON DELETE RESTRICT", "MD__c": "ON DELETE CASCADE"}[in.Name]
		if e != nil || !strings.Contains(rule, wantRule) {
			t.Errorf("%s constraint %q %v want %q", in.Name, rule, e, wantRule)
		}
	}
	_ = other
}

func TestMasterDetailRequiresEmpty(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "md")
	o, _ := CreateObject(ctx, p, "md", ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	CreateObject(ctx, p, "md", ObjectInput{Name: "Task__c", Label: "Task", PluralLabel: "Tasks"})
	tx, _ := p.Begin(ctx)
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id','md',true)")
	if _, e := tx.Exec(ctx, "INSERT INTO "+o.TableSQL()+"(tenant_id,id,name) VALUES('md','a00000000000001AAA','x')"); e != nil {
		t.Fatal(e)
	}
	tx.Commit(ctx)
	before := revision(t, p, "md")
	if _, e := CreateField(ctx, p, "md", FieldInput{Object: "Project__c", Name: "Task__c", Label: "Task", Type: "MasterDetail", ReferenceTo: "Task__c"}); e == nil {
		t.Fatal("master-detail on non-empty object accepted")
	}
	var n int
	p.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_name=$1", o.Table).Scan(&n)
	if n != 3 || revision(t, p, "md") != before {
		t.Fatalf("partial change: %d columns, revision %d→%d", n, before, revision(t, p, "md"))
	}
}

func revision(t *testing.T, p *pgxpool.Pool, tenant string) int64 {
	t.Helper()
	var r int64
	if e := p.QueryRow(context.Background(), "SELECT revision FROM sc_catalog.revisions WHERE tenant_id=$1", tenant).Scan(&r); e != nil {
		t.Fatal(e)
	}
	return r
}

func TestAutoNumberBackfill(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "an")
	o, _ := CreateObject(ctx, p, "an", ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	tx, _ := p.Begin(ctx)
	tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id','an',true)")
	for _, id := range []string{"a00000000000003AAA", "a00000000000001AAA", "a00000000000002AAA"} {
		if _, e := tx.Exec(ctx, "INSERT INTO "+o.TableSQL()+"(tenant_id,id,name) VALUES('an',$1,'x')", id); e != nil {
			t.Fatal(e)
		}
	}
	tx.Commit(ctx)
	f, e := CreateField(ctx, p, "an", FieldInput{Object: "Project__c", Name: "Num__c", Label: "Num", Type: "AutoNumber", DisplayFormat: "N-{00}", StartingNumber: 5})
	if e != nil {
		t.Fatal(e)
	}
	rows, _ := p.Query(ctx, "SELECT id||'='||"+f.Column+" FROM "+o.TableSQL()+" ORDER BY id")
	var got []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		got = append(got, v)
	}
	if strings.Join(got, ",") != "a00000000000001AAA=N-05,a00000000000002AAA=N-06,a00000000000003AAA=N-07" {
		t.Fatalf("backfill %v", got)
	}
	var next int64
	if e := p.QueryRow(ctx, "SELECT next_value FROM sc_catalog.auto_numbers WHERE field_id=$1", f.ID).Scan(&next); e != nil || next != 8 {
		t.Fatalf("counter %d %v", next, e)
	}
	if !f.AutoNumber || f.Createable || f.Updateable || f.Nillable {
		t.Fatalf("describe %#v", f)
	}
}

func TestAutoNumberObject(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "ao")
	o, e := CreateObject(ctx, p, "ao", ObjectInput{Name: "Ticket__c", Label: "Ticket", PluralLabel: "Tickets", NameType: "AutoNumber", DisplayFormat: "T-{0000}", StartingNumber: 1})
	if e != nil {
		t.Fatal(e)
	}
	name := o.Fields[1]
	if name.Name != "Name" || !name.AutoNumber || name.Createable || !name.NameField || name.Length != 30 {
		t.Fatalf("auto-number name %#v", name)
	}
	var unique bool
	p.QueryRow(ctx, "SELECT EXISTS(SELECT FROM pg_indexes WHERE schemaname='sc_records' AND tablename=$1 AND indexdef LIKE '%UNIQUE%(tenant_id, name)%')", o.Table).Scan(&unique)
	if !unique {
		t.Fatal("auto-number Name is not unique per tenant")
	}
	if _, e := CreateObject(ctx, p, "ao", ObjectInput{Name: "Bad__c", Label: "Bad", PluralLabel: "Bads", NameType: "AutoNumber", DisplayFormat: "none"}); e == nil {
		t.Fatal("invalid display format accepted")
	}
	if _, e := CreateObject(ctx, p, "ao", ObjectInput{Name: "Bad__c", Label: "Bad", PluralLabel: "Bads", NameType: "Number"}); e == nil {
		t.Fatal("unsupported name type accepted")
	}
}

func TestStoredFieldReservesName(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "st")
	o, _ := CreateObject(ctx, p, "st", ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	def := []byte(`{"name":"Score__c","label":"Score","type":"Formula","formula":"Amount__c * 2","returnType":"Number"}`)
	sf, e := CreateStoredField(ctx, p, "st", "Project__c", def)
	if e != nil || sf.Name != "Score__c" || sf.Kind != "Formula" {
		t.Fatal(sf, e)
	}
	if _, e := CreateField(ctx, p, "st", FieldInput{Object: "Project__c", Name: "score__c", Label: "S", Type: "Text", Length: 5}); e == nil {
		t.Fatal("active field took a stored name")
	}
	if _, e := CreateStoredField(ctx, p, "st", "Project__c", []byte(`{"name":"X__c","label":"X","type":"Text"}`)); e == nil {
		t.Fatal("active type stored as not enforced")
	}
	if _, e := CreateStoredField(ctx, p, "st", "Project__c", []byte(`{"name":"SCORE__c","label":"Dup","type":"RollupSummary"}`)); e == nil {
		t.Fatal("duplicate stored name accepted")
	}
	tx, _ := p.Begin(ctx)
	defer tx.Rollback(ctx)
	snap, e := Load(ctx, tx, "st")
	if e != nil || len(snap.Stored) != 1 || snap.Stored[0].Object != "Project__c" {
		t.Fatalf("stored %v %#v", e, snap.Stored)
	}
	if _, e := snap.Field("Project__c", "Score__c"); e == nil {
		t.Fatal("stored definition is described")
	}
	var n int
	p.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_name=$1", o.Table).Scan(&n)
	if n != 3 {
		t.Fatalf("stored definition created a column: %d", n)
	}
	var source []byte
	p.QueryRow(ctx, "SELECT source FROM sc_catalog.artifacts WHERE api_name='Score__c'").Scan(&source)
	if string(source) != string(def) {
		t.Fatal("source definition not retained exactly")
	}
	raw, _ := json.Marshal(Inspect(snap))
	if !strings.Contains(string(raw), `"status":"stored-not-enforced"`) || !strings.Contains(string(raw), `"status":"active"`) {
		t.Fatalf("inspect statuses missing: %s", raw)
	}
}

func TestSchemaCurrency(t *testing.T) {
	p := catalogPool(t)
	ctx := context.Background()
	seed(t, p, "cur")
	if e := SetCurrency(ctx, p, "cur", "EUR"); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{"eur", "EURO", ""} {
		if e := SetCurrency(ctx, p, "cur", bad); e == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if e := SetCurrency(ctx, p, "missing", "EUR"); e == nil {
		t.Error("unprovisioned tenant accepted")
	}
	tx, _ := p.Begin(ctx)
	defer tx.Rollback(ctx)
	snap, _ := Load(ctx, tx, "cur")
	if snap.Currency != "EUR" {
		t.Fatalf("currency %q", snap.Currency)
	}
}

func TestSharedStandardTableRules(t *testing.T) {
	s, _ := specSnapshot()
	account, _ := s.Object("Account")
	for _, in := range []FieldInput{
		{Name: "Req__c", Label: "R", Type: "Text", Length: 5, Required: true},
		{Name: "Seq__c", Label: "S", Type: "AutoNumber", DisplayFormat: "S-{0}"},
	} {
		if _, _, err := BuildField(in, account, s, false); err == nil {
			t.Errorf("%s on shared standard table accepted", in.Type)
		}
	}
	if _, _, err := BuildField(FieldInput{Name: "Ok__c", Label: "O", Type: "Text", Length: 5}, account, s, false); err != nil {
		t.Errorf("optional field on standard object rejected: %v", err)
	}
}

func TestRequiredLookupRestricts(t *testing.T) {
	s, project := specSnapshot()
	f, ddl, err := BuildField(FieldInput{Name: "Acct__c", Label: "A", Type: "Lookup", ReferenceTo: "Account", Required: true}, project, s, false)
	if err != nil || f.Options.DeleteBehavior != "Restrict" || !strings.Contains(ddl.ForeignKey, "ON DELETE RESTRICT") {
		t.Fatalf("required lookup %v %q %q", err, f.Options.DeleteBehavior, ddl.ForeignKey)
	}
	if _, _, err := BuildField(FieldInput{Name: "Acct__c", Label: "A", Type: "Lookup", ReferenceTo: "Account", Required: true, DeleteBehavior: "SetNull"}, project, s, false); err == nil {
		t.Fatal("required SetNull lookup accepted")
	}
}

func TestChildRelationshipDefaults(t *testing.T) {
	if got := DefaultChildRelationshipName(Object{Name: "Typed__c", Label: "Typed", PluralLabel: "Typed items"}); got != "Typeditems__r" {
		t.Fatalf("default name %q", got)
	}
	if got := DefaultChildRelationshipName(Object{Name: "Contact", Label: "Contact", PluralLabel: "Contacts"}); got != "Contacts" {
		t.Fatalf("standard default %q", got)
	}
	s, project := specSnapshot()
	f, _, err := BuildField(FieldInput{Name: "A__c", Label: "A", Type: "Lookup", ReferenceTo: "Account", ChildRelationshipName: "Foos"}, project, s, false)
	if err != nil || f.Options.ChildRelationshipName != "Foos" {
		t.Fatalf("explicit name %v %#v", err, f.Options)
	}
	f, _, err = BuildField(FieldInput{Name: "B__c", Label: "B", Type: "Lookup", ReferenceTo: "Account"}, project, s, false)
	if err != nil || f.Options.ChildRelationshipName != "Projects__r" {
		t.Fatalf("default name %v %#v", err, f.Options)
	}
	if _, _, err := BuildField(FieldInput{Name: "C__c", Label: "C", Type: "Lookup", ReferenceTo: "Account", ChildRelationshipName: "bad name"}, project, s, false); err == nil {
		t.Fatal("invalid child relationship name accepted")
	}
	if _, _, err := BuildField(FieldInput{Name: "C__c", Label: "C", Type: "Text", Length: 5, ChildRelationshipName: "Foos"}, project, s, false); err == nil {
		t.Fatal("child relationship name on Text accepted")
	}
	f1, _, err := BuildField(FieldInput{Name: "D__c", Label: "D", Type: "Lookup", ReferenceTo: "Account", ChildRelationshipName: "Dup"}, project, s, false)
	if err != nil {
		t.Fatal(err)
	}
	s.Objects = append(s.Objects, Object{Name: "Other__c", Label: "Other", PluralLabel: "Others", Fields: []Field{f1}})
	if _, _, err := BuildField(FieldInput{Name: "E__c", Label: "E", Type: "Lookup", ReferenceTo: "Account", ChildRelationshipName: "dup"}, project, s, false); err == nil {
		t.Fatal("duplicate child relationship name accepted")
	}
	if _, _, err := BuildField(FieldInput{Name: "F__c", Label: "F", Type: "Lookup", ReferenceTo: "Project__c", ChildRelationshipName: "Dup"}, project, s, false); err != nil {
		t.Fatalf("name reused on another parent: %v", err)
	}
	typed := Object{Name: "Typed__c", Label: "Typed", PluralLabel: "Typed items", Fields: []Field{
		{Name: "Id", Options: Options{Kind: fieldtype.Id}},
		Complete(Field{Name: "Account__c", Label: "Account", Nillable: true, Length: 18, ReferenceTo: []string{"Account"}, RelationshipName: ptr("Account__r"),
			Options: Options{Kind: fieldtype.Lookup, DeleteBehavior: "Restrict", ChildRelationshipName: "Typeditems__r"}}),
	}}
	s2 := Snapshot{Tenant: "t", Objects: append(StandardObjects(), typed)}
	if got := s2.ChildRelationships("Account"); len(got) != 2 {
		t.Fatalf("children %#v", got)
	}
	o, fld, err := s2.ChildRelationship("account", "typeditems__r")
	if err != nil || o.Name != "Typed__c" || fld.Name != "Account__c" {
		t.Fatalf("resolve %v %#v %#v", err, o.Name, fld.Name)
	}
	if _, _, err := s2.ChildRelationship("Account", "Nope"); err == nil {
		t.Fatal("unknown child relationship resolved")
	}
}

func TestDescribeChildRelationships(t *testing.T) {
	master := Object{Name: "Master__c", Label: "Master", PluralLabel: "Masters", Fields: []Field{
		{Name: "Id", Options: Options{Kind: fieldtype.Id}},
	}}
	detail := Object{Name: "Detail__c", Label: "Detail", PluralLabel: "Details", Fields: []Field{
		{Name: "Id", Options: Options{Kind: fieldtype.Id}},
		Complete(Field{Name: "Master__c", Label: "Master", Length: 18, ReferenceTo: []string{"Master__c"}, RelationshipName: ptr("Master__r"),
			Options: Options{Kind: fieldtype.MasterDetail, DeleteBehavior: "Cascade", ChildRelationshipName: "Details__r"}}),
	}}
	s := Snapshot{Tenant: "t", Objects: append(StandardObjects(), master, detail)}
	p := records.Principal{Role: "admin", Catalog: &s}
	d, err := records.Describe(p, "Account")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cr := range d.ChildRelationships {
		if cr.ChildSObject == "Contact" && cr.Field == "AccountId" {
			found = true
			if cr.RelationshipName != "Contacts" || !cr.RestrictedDelete || cr.CascadeDelete || cr.DeprecatedAndHidden {
				t.Fatalf("Contacts %#v", cr)
			}
		}
	}
	if !found {
		t.Fatalf("Account children %#v", d.ChildRelationships)
	}
	d, err = records.Describe(p, "Master__c")
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, cr := range d.ChildRelationships {
		if cr.ChildSObject == "Detail__c" && cr.Field == "Master__c" {
			found = true
			if cr.RelationshipName != "Details__r" || !cr.CascadeDelete || cr.RestrictedDelete {
				t.Fatalf("Details %#v", cr)
			}
		}
	}
	if !found {
		t.Fatalf("Master children %#v", d.ChildRelationships)
	}
	// The restricted role does not see children through a denied field.
	denied := Object{Name: "Account", Label: "Account", PluralLabel: "Accounts", Fields: []Field{
		{Name: "Id", Options: Options{Kind: fieldtype.Id}},
		Complete(Field{Name: "Compat_Text__c", Label: "Compat", Nillable: true, Length: 18, ReferenceTo: []string{"Group__c"}, RelationshipName: ptr("Compat_Text__r"),
			Options: Options{Kind: fieldtype.Lookup, DeleteBehavior: "SetNull", ChildRelationshipName: "Accounts"}}),
	}}
	group := Object{Name: "Group__c", Label: "Group", PluralLabel: "Groups", Fields: []Field{
		{Name: "Id", Options: Options{Kind: fieldtype.Id}},
	}}
	sg := Snapshot{Tenant: "t", Objects: []Object{group, denied}}
	admin, aErr := records.Describe(records.Principal{Role: "admin", Catalog: &sg}, "Group__c")
	if aErr != nil || len(admin.ChildRelationships) != 1 {
		t.Fatalf("admin children %#v %v", admin.ChildRelationships, aErr)
	}
	rest, rErr := records.Describe(records.Principal{Role: "restricted", Catalog: &sg}, "Group__c")
	if rErr != nil || len(rest.ChildRelationships) != 0 {
		t.Fatalf("restricted children %#v %v", rest.ChildRelationships, rErr)
	}
}
