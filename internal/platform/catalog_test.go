package platform

import (
	"strings"
	"testing"
)

func TestValidateWrites(t *testing.T) {
	for _, tt := range []struct {
		name, object, role string
		data               map[string]any
		create, ok         bool
	}{
		{"valid", "Account", "admin", map[string]any{"Name": "Acme"}, true, true},
		{"missing", "Account", "admin", map[string]any{}, true, false},
		{"null required", "Account", "admin", map[string]any{"Name": nil}, false, false},
		{"empty required", "Contact", "admin", map[string]any{"LastName": " "}, true, false},
		{"numeric", "Account", "admin", map[string]any{"Name": 42}, true, false},
		{"unknown", "Account", "admin", map[string]any{"Name": "A", "tenant": "other"}, true, false},
		{"readonly", "Account", "admin", map[string]any{"Id": "001"}, false, false},
		{"nullable", "Account", "admin", map[string]any{"Compat_Text__c": nil}, false, true},
		{"denied", "Account", "restricted", map[string]any{"Compat_Text__c": "secret"}, false, false},
		{"unknown object", "Bogus", "admin", map[string]any{}, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(Principal{Role: tt.role}, tt.object, tt.data, tt.create)
			if (err == nil) != tt.ok {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func TestDescribeHidesDeniedField(t *testing.T) {
	d, err := Describe(Principal{Role: "restricted"}, "Account")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range d.Fields {
		if f.Name == "Compat_Text__c" {
			t.Fatal("denied field exposed")
		}
	}
}

func TestCatalogDeclaredLengths(t *testing.T) {
	d, err := Describe(Principal{Role: "admin"}, "Account")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range d.Fields {
		if f.Name == "Compat_Text__c" || f.Name == "Compat_Key__c" {
			if f.Length != 80 || f.Type != "string" || !f.Nillable || len(f.ReferenceTo) != 0 || f.ExternalID != (f.Name == "Compat_Key__c") {
				t.Fatalf("unexpected field %+v", f)
			}
		} else if f.Name == "Name" && f.Length != 255 {
			t.Fatal(f)
		}
	}
	for _, lookup := range []func(string) (CustomFieldDeclaration, bool){
		func(string) (CustomFieldDeclaration, bool) { return CustomFieldDeclaration{}, false },
		func(n string) (CustomFieldDeclaration, bool) {
			d, _ := DeclaredCustomField(n)
			d.Length = 0
			return d, true
		},
	} {
		if _, err := buildCatalogWith(lookup); err == nil {
			t.Fatal("invalid declaration accepted")
		}
	}
}
func TestCustomLengthBoundaries(t *testing.T) {
	for _, name := range []string{"Compat_Text__c", "Compat_Key__c"} {
		for _, char := range []string{"x", "é", "😀"} {
			for _, n := range []int{80, 81} {
				err := Validate(Principal{Role: "admin"}, "Account", map[string]any{name: strings.Repeat(char, n)}, false)
				if n == 80 && err != nil {
					t.Fatal(err)
				}
				if n == 81 {
					e, ok := err.(*APIError)
					if !ok || e.Code != "STRING_TOO_LONG" {
						t.Fatalf("length %d: %v", n, err)
					}
				}
			}
		}
		for _, v := range []any{nil, ""} {
			if err := Validate(Principal{Role: "admin"}, "Account", map[string]any{name: v}, false); err != nil {
				t.Fatal(err)
			}
		}
	}
}
