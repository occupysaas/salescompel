package platform

import (
	"testing"
)

// Organization is a read-only catalog object over the tenant settings,
// readable by every API-enabled user (the CLI's discovery query runs as any
// user) and refused on write (D33-31).
func TestOrganizationQueryable(t *testing.T) {
	f := newWriteFixture(t)
	standard := f.newUser("org", "Standard User")
	for _, who := range []readFixture{f.readFixture, standard} {
		code, body := who.query("SELECT Id, Name, TimeZoneSidKey, DefaultLocaleSidKey, LanguageLocaleKey, DefaultCurrencyIsoCode FROM Organization")
		m, _ := body.(map[string]any)
		records, _ := m["records"].([]any)
		if code != 200 || len(records) != 1 {
			t.Fatalf("Organization query: %d %v", code, body)
		}
		r := records[0].(map[string]any)
		if r["TimeZoneSidKey"] != "America/Los_Angeles" || r["DefaultLocaleSidKey"] != "en_US" || r["LanguageLocaleKey"] != "en_US" || r["DefaultCurrencyIsoCode"] != "USD" || r["Name"] != f.tenant {
			t.Fatalf("Organization record %v", r)
		}
		if r["Id"] != orgID(f.tenant) {
			t.Fatalf("Organization Id %v, want the token's %s", r["Id"], orgID(f.tenant))
		}
	}
	// The pinned CLI's discovery query is an ordinary query now.
	code, body := standard.query("SELECT Name, InstanceName, IsSandbox, TrialExpirationDate, NamespacePrefix, OrganizationType FROM Organization")
	m, _ := body.(map[string]any)
	records, _ := m["records"].([]any)
	if code != 200 || len(records) != 1 || records[0].(map[string]any)["IsSandbox"] != false || records[0].(map[string]any)["OrganizationType"] != "SalesCompel" {
		t.Fatalf("discovery query: %d %v", code, body)
	}
	id := f.one("SELECT Id FROM Organization", "Id")
	code, body = f.do("PATCH", "sobjects/Organization/"+id, `{"TimeZoneSidKey":"Europe/Paris"}`, false)
	wantFailure(t, "Organization update", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
	code, body = f.do("POST", "sobjects/Organization", `{"Name":"Another"}`, false)
	wantFailure(t, "Organization create", code, body, 403, "INSUFFICIENT_ACCESS_OR_READONLY")
}
