package platform

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

type columnsContractCase struct {
	ID             string          `json:"id"`
	Principal      string          `json:"principal"`
	Query          string          `json:"query"`
	QuerySuffix    string          `json:"querySuffix"`
	ExpectedStatus int             `json:"expectedStatus"`
	ExpectedBody   json.RawMessage `json:"expectedBody"`
	Basis          []string        `json:"basis"`
}

func columnsCases(t *testing.T) []columnsContractCase {
	t.Helper()
	b, err := os.ReadFile("testdata/query-columns-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion int                   `json:"schemaVersion"`
		Cases         []columnsContractCase `json:"cases"`
	}
	if err = json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.SchemaVersion != 1 || len(fixture.Cases) != 35 {
		t.Fatal("incomplete contract oracle")
	}
	return fixture.Cases
}
func TestQueryColumnsOptions(t *testing.T) {
	for _, tt := range []struct {
		raw       string
		want, bad bool
	}{
		{"", false, false}, {"columns=true", true, false}, {"columns=false", false, false},
		{"columns=", false, true}, {"columns=TRUE", false, true}, {"columns=1", false, true},
		{"columns=true&columns=false", false, true}, {"columns=false&columns=true", false, true},
		{"columns=true&columns=true", false, true}, {"other=true", false, false},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			values, err := url.ParseQuery(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := queryColumnsRequested(values)
			if (err != nil) != tt.bad || got != tt.want {
				t.Fatalf("got %v %v, want enabled=%v bad=%v", got, err, tt.want, tt.bad)
			}
		})
	}
}
func TestHTTPQueryColumns(t *testing.T)       { runColumnsContract(t, false) }
func TestHTTPQueryColumnsPolicy(t *testing.T) { runColumnsContract(t, true) }
func runColumnsContract(t *testing.T, policy bool) {
	s := testStore(t)
	a, b := principals(t, s)
	ctx := context.Background()
	create := func(p Principal, obj string, data map[string]any) string {
		t.Helper()
		id, err := s.Create(ctx, p, obj, data)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	name := "Plan16 oracle"
	account := create(a, "Account", map[string]any{"Name": name, "Compat_Key__c": "owned-key"})
	contact := create(a, "Contact", map[string]any{"LastName": "Contact", "AccountId": account})
	nullContact := create(a, "Contact", map[string]any{"LastName": "Unlinked"})
	create(b, "Account", map[string]any{"Name": name})
	restricted := a
	restricted.Role = "restricted"
	restricted.User = "reader"
	token := "restricted-" + a.Tenant
	if err := s.Provision(ctx, token, restricted); err != nil {
		t.Fatal(err)
	}
	tokens := map[string]string{"admin": "token-" + a.Tenant, "restricted": token, "other": "token-" + b.Tenant}
	replace := strings.NewReplacer("$accountId", account, "$contactId", contact, "$nullContactId", nullContact, "$name", name)
	h := NewHandler(s, "")
	// Warm an administrator projection before restricted requests to catch cross-principal metadata reuse.
	warm := httptest.NewRequest("GET", "/services/data/v65.0/query?q=SELECT+Id,Name,Compat_Text__c+FROM+Account&columns=true", nil)
	warm.Header.Set("Authorization", "Bearer "+tokens["admin"])
	warmed := httptest.NewRecorder()
	h.ServeHTTP(warmed, warm)
	if warmed.Code != 200 || !strings.Contains(warmed.Body.String(), `"columnName":"Compat_Text__c"`) {
		t.Fatalf("administrator metadata: %d %s", warmed.Code, warmed.Body.String())
	}
	for _, c := range columnsCases(t) {
		isPolicy := c.Principal != "admin" || (c.ExpectedStatus != 200 && !strings.HasPrefix(c.ID, "option-"))
		if isPolicy != policy {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			if len(c.Basis) == 0 {
				t.Fatal("missing provenance")
			}
			r := httptest.NewRequest("GET", "/services/data/v65.0/query?q="+url.QueryEscape(replace.Replace(c.Query))+c.QuerySuffix, nil)
			r.Header.Set("Authorization", "Bearer "+tokens[c.Principal])
			r.Header.Set("X-Tenant-ID", b.Tenant)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var got, want any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(replace.Replace(string(c.ExpectedBody))), &want); err != nil {
				t.Fatal(err)
			}
			if w.Code != c.ExpectedStatus || !reflect.DeepEqual(got, want) {
				t.Fatalf("status %d want %d\nbody %s\nwant %s", w.Code, c.ExpectedStatus, w.Body.String(), replace.Replace(string(c.ExpectedBody)))
			}
		})
	}
	// Existing empty database behavior must also carry projection metadata.
	if !policy {
		_, empty := principals(t, s)
		r := httptest.NewRequest("GET", "/services/data/v65.0/query?q=SELECT+Id+FROM+Account&columns=true", nil)
		r.Header.Set("Authorization", "Bearer token-"+empty.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var got any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		var want any
		_ = json.Unmarshal([]byte(`{"totalSize":0,"done":true,"records":[],"columnMetadata":[{"columnName":"Id"}]}`), &want)
		if w.Code != 200 || !reflect.DeepEqual(got, want) {
			t.Fatalf("empty database: %d %s", w.Code, w.Body.String())
		}
	}
}
