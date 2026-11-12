package main

import (
	"os"
	"path/filepath"
	"reflect"
	"salescompel/internal/metadata"
	"testing"
)

func TestSchemaFieldFlags(t *testing.T) {
	req, err := parseSchemaArgs("field", []string{"--tenant", "t", "--object", "Project__c", "--name", "Stage__c", "--label", "Stage", "--type", "Picklist", "--values", "New,Won", "--restricted", "--default", "New"})
	if err != nil {
		t.Fatal(err)
	}
	def := "New"
	want := metadata.FieldInput{Object: "Project__c", Name: "Stage__c", Label: "Stage", Type: "Picklist", Restricted: true, Default: &def,
		Values: []metadata.PicklistValue{{Value: "New", Label: "New", Active: true}, {Value: "Won", Label: "Won", Active: true}}}
	if req.Tenant != "t" || !reflect.DeepEqual(req.Field, want) {
		t.Fatalf("got %#v", req.Field)
	}
	req, err = parseSchemaArgs("field", []string{"--tenant", "t", "--object", "O__c", "--name", "N__c", "--label", "N", "--type", "AutoNumber", "--display-format", "N-{0}"})
	if err != nil || req.Field.StartingNumber != 1 {
		t.Fatalf("auto-number default start: %v %#v", err, req.Field)
	}
	req, err = parseSchemaArgs("field", []string{"--tenant", "t", "--object", "O__c", "--name", "R__c", "--label", "R", "--type", "Lookup", "--reference-to", "Account", "--delete-behavior", "Restrict", "--relationship-name", "Acct__r", "--required"})
	if err != nil || req.Field.ReferenceTo != "Account" || req.Field.DeleteBehavior != "Restrict" || req.Field.RelationshipName != "Acct__r" || !req.Field.Required {
		t.Fatalf("lookup flags: %v %#v", err, req.Field)
	}
	for name, args := range map[string][]string{
		"unknown flag":       {"--tenant", "t", "--object", "O__c", "--name", "N__c", "--label", "N", "--type", "Text", "--length", "5", "--colour", "red"},
		"duplicate flag":     {"--tenant", "t", "--tenant", "u", "--object", "O__c", "--name", "N__c", "--label", "N", "--type", "Text"},
		"definition + type":  {"--tenant", "t", "--object", "O__c", "--definition", "x.json", "--type", "Text"},
		"plural on field":    {"--tenant", "t", "--object", "O__c", "--name", "N__c", "--label", "N", "--type", "Text", "--plural-label", "Ns"},
		"no tenant":          {"--object", "O__c", "--name", "N__c", "--label", "N", "--type", "Text"},
		"positional":         {"--tenant", "t", "extra"},
		"name type on field": {"--tenant", "t", "--object", "O__c", "--name", "N__c", "--label", "N", "--type", "Text", "--name-type", "AutoNumber"},
	} {
		if _, err := parseSchemaArgs("field", args); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	obj, err := parseSchemaArgs("object", []string{"--tenant", "t", "--name", "Ticket__c", "--label", "Ticket", "--plural-label", "Tickets", "--name-type", "AutoNumber", "--display-format", "T-{0000}"})
	if err != nil || obj.Object.NameType != "AutoNumber" || obj.Object.DisplayFormat != "T-{0000}" || obj.Object.StartingNumber != 1 {
		t.Fatalf("object flags %v %#v", err, obj.Object)
	}
	if _, err := parseSchemaArgs("object", []string{"--tenant", "t", "--name", "X__c", "--label", "X", "--plural-label", "Xs", "--type", "Text"}); err == nil {
		t.Error("field flag accepted on object")
	}
	cur, err := parseSchemaArgs("currency", []string{"--tenant", "t", "--code", "EUR"})
	if err != nil || cur.Code != "EUR" {
		t.Fatalf("currency %v %#v", err, cur)
	}
	if _, err := parseSchemaArgs("currency", []string{"--tenant", "t", "--code", "EUR", "--name", "x"}); err == nil {
		t.Error("extra flag accepted on currency")
	}
}

func TestSchemaDefinitionFile(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "active.json")
	os.WriteFile(active, []byte(`{"name":"Stage__c","label":"Stage","type":"Picklist","restricted":true,"values":[{"value":"new","label":"New","default":true},{"value":"won","label":"Won"}]}`), 0o600)
	req, err := parseSchemaArgs("field", []string{"--tenant", "t", "--object", "O__c", "--definition", active})
	if err != nil {
		t.Fatal(err)
	}
	def := "new"
	want := metadata.FieldInput{Object: "O__c", Name: "Stage__c", Label: "Stage", Type: "Picklist", Restricted: true, Default: &def,
		Values: []metadata.PicklistValue{{Value: "new", Label: "New", Active: true}, {Value: "won", Label: "Won", Active: true}}}
	if req.Stored != nil || !reflect.DeepEqual(req.Field, want) {
		t.Fatalf("active definition %#v", req.Field)
	}
	stored := filepath.Join(dir, "stored.json")
	os.WriteFile(stored, []byte(`{"name":"Score__c","label":"Score","type":"Formula","formula":"1+1"}`), 0o600)
	req, err = parseSchemaArgs("field", []string{"--tenant", "t", "--object", "O__c", "--definition", stored})
	if err != nil || string(req.Stored) == "" {
		t.Fatalf("stored definition %v %#v", err, req)
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{"name":"X__c","label":"X","type":"Text","colour":"red"}`), 0o600)
	if _, err := parseSchemaArgs("field", []string{"--tenant", "t", "--object", "O__c", "--definition", bad}); err == nil {
		t.Error("unknown definition key accepted")
	}
}
