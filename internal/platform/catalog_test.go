package platform

import "testing"

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
