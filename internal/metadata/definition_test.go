package metadata_test

import (
	"reflect"
	"salescompel/internal/fieldtype"
	. "salescompel/internal/metadata"
	"strings"
	"testing"
)

func sampleField(k fieldtype.Kind) Field {
	f := Field{Name: "X__c", Label: "X", Nillable: true, Createable: true, Updateable: true, Options: Options{Kind: k}}
	switch {
	case k.Numeric():
		f.Precision, f.Scale = 18, 2
	case k == fieldtype.Picklist || k == fieldtype.MultiselectPicklist:
		f.PicklistValues = []PicklistValue{{Active: true, Label: "A", Value: "A"}}
	case k.Reference():
		f.ReferenceTo = []string{"Account"}
		rel := "X__r"
		f.RelationshipName = &rel
		f.Options.DeleteBehavior = "SetNull"
	case k == fieldtype.AutoNumber:
		f.Options.DisplayFormat = "A-{0000}"
		f.Createable, f.Updateable, f.Nillable = false, false, false
	case k == fieldtype.Address:
		f.Options.Components = []string{"XStreet"}
	}
	return Complete(f)
}

func TestDefinitionRoundTrip(t *testing.T) {
	for _, k := range []fieldtype.Kind{fieldtype.Id, fieldtype.Text, fieldtype.TextArea, fieldtype.LongTextArea, fieldtype.Number, fieldtype.Currency, fieldtype.Percent, fieldtype.Date, fieldtype.DateTime, fieldtype.Checkbox, fieldtype.Picklist, fieldtype.MultiselectPicklist, fieldtype.Email, fieldtype.Phone, fieldtype.Url, fieldtype.Lookup, fieldtype.MasterDetail, fieldtype.AutoNumber, fieldtype.Address, fieldtype.PersonName} {
		f := sampleField(k)
		raw, err := EncodeDefinition(f)
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		back, err := DecodeDefinition(raw)
		if err != nil || !reflect.DeepEqual(back, f) {
			t.Fatalf("%s: round trip %v\n got %#v\nwant %#v", k, err, back, f)
		}
		if f.Type != k.DescribeType() || f.SoapType != k.SoapType() || f.PicklistValues == nil || f.ReferenceTo == nil {
			t.Fatalf("%s: Complete did not derive describe properties: %#v", k, f)
		}
	}
	good, _ := EncodeDefinition(sampleField(fieldtype.Text))
	bad := []string{
		strings.Replace(string(good), `"v":2`, `"v":1`, 1),
		strings.Replace(string(good), `"options":{`, `"options":{"grant":true,`, 1),
		strings.Replace(string(good), `"kind":"Text"`, `"kind":"Formula"`, 1),
		strings.Replace(string(good), `"type":"string"`, `"type":"picklist"`, 1),
		strings.Replace(string(good), `"v":2`, `"v":2,"extra":1`, 1),
	}
	for _, b := range bad {
		if b == string(good) {
			t.Fatalf("fixture replacement did not apply: %s", b)
		}
		if _, err := DecodeDefinition([]byte(b)); err == nil {
			t.Errorf("accepted %s", b)
		}
	}
}

func TestNumberMayDescribeAsInt(t *testing.T) {
	f := Complete(Field{Name: "NumberOfEmployees", Type: "int", Digits: 8, Nillable: true, Options: Options{Kind: fieldtype.Number}})
	raw, _ := EncodeDefinition(f)
	back, err := DecodeDefinition(raw)
	if err != nil || back.Type != "int" || back.SoapType != "xsd:int" {
		t.Fatalf("int number: %v %#v", err, back)
	}
}
