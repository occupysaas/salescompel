package platform

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDeclaredCustomFields(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		external, unique, caseKnown bool
	}{
		{"Account.Compat_Text__c", false, false, false},
		{"Account.Compat_Key__c", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, known := DeclaredCustomField(tc.name)
			if !known || got.FullName != tc.name || got.Type != "Text" || got.Length != 80 || got.Required || got.ExternalID != tc.external || got.Unique != tc.unique {
				t.Fatalf("unexpected declaration: %+v, known=%v", got, known)
			}
			if got.CaseSensitive.Known != tc.caseKnown || got.CaseSensitive.Value {
				t.Fatalf("missing and explicit false conflated: %+v", got.CaseSensitive)
			}
		})
	}
	for _, name := range []string{"", "Account.Name", "Contact.Compat_Text__c", "Compat_Text__c", "account.Compat_Text__c", "Account.compat_key__c", "Account.Compat_Key__c ", "*"} {
		if got, known := DeclaredCustomField(name); known || got != (CustomFieldDeclaration{}) {
			t.Errorf("unknown %q returned %+v, %v", name, got, known)
		}
	}
}

func TestDeclaredFieldGrants(t *testing.T) {
	for _, field := range []string{"Account.Compat_Text__c", "Account.Compat_Key__c"} {
		got, known := DeclaredFieldGrant("Compat_Admin", field)
		if !known || got.PermissionSet != "Compat_Admin" || got.Field != field || !got.Readable || !got.Editable {
			t.Fatalf("unexpected declared grant: %+v, known=%v", got, known)
		}
	}
	for _, tc := range [][2]string{
		{"", "Account.Compat_Key__c"}, {"admin", "Account.Compat_Key__c"},
		{"compat_admin", "Account.Compat_Key__c"}, {"Unknown", "Account.Compat_Key__c"},
		{"Compat_Admin", "Account.Name"}, {"Compat_Admin", "Contact.AccountId"},
		{"Compat_Admin", "Account"}, {"Compat_Admin", "account.Compat_Key__c"},
	} {
		if got, known := DeclaredFieldGrant(tc[0], tc[1]); known || got != (FieldGrantDeclaration{}) {
			t.Fatalf("unknown grant %v returned %+v, %v", tc, got, known)
		}
	}
}

func TestDeclaredValuesAreIsolated(t *testing.T) {
	first, _ := DeclaredCustomField("Account.Compat_Key__c")
	original := first
	first.Length = 1
	first.CaseSensitive.Value = true
	first.FullName = "changed"
	next, _ := DeclaredCustomField("Account.Compat_Key__c")
	if next != original {
		t.Fatal("caller mutated field declaration")
	}
	grant, _ := DeclaredFieldGrant("Compat_Admin", "Account.Compat_Key__c")
	expected := grant
	grant.Readable = false
	grant.Field = "changed"
	again, _ := DeclaredFieldGrant("Compat_Admin", "Account.Compat_Key__c")
	if again != expected {
		t.Fatal("caller mutated declared grant")
	}
}

func TestDeclaredMetadataDoesNotActivatePermissionsOrChangeCatalog(t *testing.T) {
	before, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	p := Principal{Role: "restricted"}
	describeBefore, err := Describe(p, "Account")
	if err != nil {
		t.Fatal(err)
	}
	// A declaration is not a permission-set assignment to this principal.
	DeclaredFieldGrant("Compat_Admin", "Account.Compat_Text__c")
	DeclaredCustomField("Account.Compat_Text__c")
	if err := Validate(p, "Account", map[string]any{"Compat_Text__c": "value"}, false); err == nil {
		t.Fatal("declared grant activated effective access")
	}
	describeAfter, err := Describe(p, "Account")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(describeBefore, describeAfter) {
		t.Fatal("declaration lookup changed describe")
	}
	after, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("declaration lookup changed persisted catalog")
	}
	// Existing candidate length and visibility behavior are not silently migrated.
	if err := Validate(p, "Account", map[string]any{"Compat_Key__c": strings.Repeat("x", 81)}, false); err != nil {
		t.Fatal("existing key access changed", err)
	}
}
