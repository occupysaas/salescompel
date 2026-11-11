package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func bodyValidationRequest(t *testing.T, h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func assertBodyValidationEnvelope(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	var items []map[string]any
	if w.Code != 400 || json.Unmarshal(w.Body.Bytes(), &items) != nil || len(items) != 1 {
		t.Fatalf("expected one 400 error item, got %d %s", w.Code, w.Body.String())
	}
	item := items[0]
	if _, present := item["fields"]; present {
		t.Errorf("fields must be absent: %s", w.Body.String())
	}
	wantMessage := "External ID value must be supplied in the upsert URL"
	if code == "JSON_PARSER_ERROR" {
		wantMessage = "External ID requires a scalar value or null"
	}
	if len(item) != 2 || item["errorCode"] != code || item["message"] != wantMessage {
		t.Errorf("error item = %v, want message and %s only", item, code)
	}
}

func TestHTTPUpsertBodyValidation(t *testing.T) {
	s := testStore(t)
	h := NewHandler(s, "")
	path := apiBase + "sobjects/Account/Compat_Key__c/body-key"
	for _, secondTenant := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			for _, tc := range upsertBodyCases() {
				t.Run(fmt.Sprintf("tenantB=%v/existing=%v/%s", secondTenant, existing, tc.name), func(t *testing.T) {
					p, peer, oldID := upsertBodyFixture(t, s, existing, secondTenant)
					before, peerBefore := accountSnapshot(t, s, p), accountSnapshot(t, s, peer)
					data := map[string]any{"Name": "Requested"}
					if tc.present {
						data["Compat_Key__c"] = tc.value
					}
					inputBefore := stringJSON(data)
					w := bodyValidationRequest(t, h, "PATCH", path, inputBefore, "token-"+p.Tenant)
					if stringJSON(data) != inputBefore {
						t.Error("caller input changed")
					}
					if tc.code != "" {
						if after := accountSnapshot(t, s, p); after != before {
							t.Errorf("rejected write changed rows: %s -> %s", before, after)
						}
						assertBodyValidationEnvelope(t, w, tc.code)
					} else {
						wantStatus := 201
						if existing {
							wantStatus = 200
						}
						var result map[string]any
						if w.Code != wantStatus || json.Unmarshal(w.Body.Bytes(), &result) != nil {
							t.Fatalf("status = %d, want %d: %s", w.Code, wantStatus, w.Body.String())
						}
						id, _ := result["id"].(string)
						errors, errorsOK := result["errors"].([]any)
						created, createdOK := result["created"].(bool)
						if len(result) != 4 || result["success"] != true || !errorsOK || len(errors) != 0 || !createdOK {
							t.Fatalf("invalid success response: %v", result)
						}
						assertUpsertBodySuccess(t, s, p, id, oldID, created, existing)
						// A successful create must become an update of the same ID.
						if !existing {
							w = bodyValidationRequest(t, h, "PATCH", path, inputBefore, "token-"+p.Tenant)
							var updated map[string]any
							if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &updated) != nil || updated["id"] != id || updated["created"] != false || updated["success"] != true {
								t.Fatalf("second upsert must update same ID: %d %s", w.Code, w.Body.String())
							}
							assertUpsertBodySuccess(t, s, p, id, id, false, false)
						}
					}
					if after := accountSnapshot(t, s, peer); after != peerBefore {
						t.Errorf("peer tenant changed: %s -> %s", peerBefore, after)
					}
				})
			}
		}
	}
}

func TestHTTPUpsertBodyBoundaryControls(t *testing.T) {
	s := testStore(t)
	p, peer, _ := upsertBodyFixture(t, s, true, false)
	h := NewHandler(s, "")
	path := apiBase + "sobjects/Account/Compat_Key__c/body-key"
	before, peerBefore := accountSnapshot(t, s, p), accountSnapshot(t, s, peer)
	for _, tc := range []struct {
		name, body, token, code string
		status                  int
	}{
		{"malformed", `{"Name":`, "token-" + p.Tenant, "JSON_PARSER_ERROR", 400},
		{"trailing-json", `{"Name":"Requested"} {}`, "token-" + p.Tenant, "JSON_PARSER_ERROR", 400},
		{"null-document", `null`, "token-" + p.Tenant, "JSON_PARSER_ERROR", 400},
		{"unauthorized", `{"Compat_Key__c":"body-key"}`, "", "INVALID_SESSION_ID", 401},
		{"invalid-token", `{"Compat_Key__c":"body-key"}`, "bad", "INVALID_SESSION_ID", 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := bodyValidationRequest(t, h, "PATCH", path, tc.body, tc.token)
			var items []map[string]any
			if w.Code != tc.status || json.Unmarshal(w.Body.Bytes(), &items) != nil || len(items) != 1 || items[0]["errorCode"] != tc.code {
				t.Errorf("got %d %s, want %d %s", w.Code, w.Body.String(), tc.status, tc.code)
			}
			if accountSnapshot(t, s, p) != before || accountSnapshot(t, s, peer) != peerBefore {
				t.Error("boundary rejection changed rows")
			}
		})
	}
}
