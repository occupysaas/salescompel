package fieldtype

import "testing"

func TestKindTable(t *testing.T) {
	want := map[Kind][2]string{
		Id: {"id", "tns:ID"}, Text: {"string", "xsd:string"}, TextArea: {"textarea", "xsd:string"}, LongTextArea: {"textarea", "xsd:string"},
		Number: {"double", "xsd:double"}, Currency: {"currency", "xsd:double"}, Percent: {"percent", "xsd:double"},
		Date: {"date", "xsd:date"}, DateTime: {"datetime", "xsd:dateTime"}, Checkbox: {"boolean", "xsd:boolean"},
		Picklist: {"picklist", "xsd:string"}, MultiselectPicklist: {"multipicklist", "xsd:string"},
		Email: {"email", "xsd:string"}, Phone: {"phone", "xsd:string"}, Url: {"url", "xsd:string"},
		Lookup: {"reference", "tns:ID"}, MasterDetail: {"reference", "tns:ID"}, AutoNumber: {"string", "xsd:string"},
		Address: {"address", "urn:address"}, PersonName: {"string", "xsd:string"},
	}
	for k, w := range want {
		if !k.Active() || k.DescribeType() != w[0] || k.SoapType() != w[1] {
			t.Errorf("%s: active=%v %s/%s want %v", k, k.Active(), k.DescribeType(), k.SoapType(), w)
		}
	}
	for _, k := range []Kind{Formula, RollupSummary, EncryptedText, Geolocation, RichTextArea} {
		if k.Active() || k.DescribeType() != "" {
			t.Errorf("%s should be stored-only", k)
		}
	}
	for _, k := range []Kind{LongTextArea, MultiselectPicklist, Address} {
		if k.Filterable() || k.Sortable() {
			t.Errorf("%s should not filter/sort", k)
		}
	}
	if !Number.Sortable() || !Picklist.Filterable() {
		t.Error("ordinary kinds must filter/sort")
	}
	if k, err := ParseKind("MultiselectPicklist"); err != nil || k != MultiselectPicklist {
		t.Errorf("ParseKind = %v %v", k, err)
	}
	for _, bad := range []string{"Id", "Address", "PersonName", "text", "Time"} {
		if _, err := ParseKind(bad); err == nil {
			t.Errorf("ParseKind(%q) accepted", bad)
		}
	}
}
