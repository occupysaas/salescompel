package platform

import (
	"context"
	"net/http/httptest"
	"salescompel/internal/metadata"
	"strings"
	"testing"
)

func TestHTTPDynamicCatalog(t *testing.T) {
	s := testStore(t)
	p, _ := principals(t, s)
	ctx := context.Background()
	o, e := metadata.CreateObject(ctx, s.pool, p.Tenant, metadata.ObjectInput{Name: "Project__c", Label: "Project", PluralLabel: "Projects"})
	if e != nil {
		t.Fatal(e)
	}
	defer s.pool.Exec(ctx, "DROP TABLE "+o.TableSQL())
	h := NewHandler(s, "")
	for _, path := range []string{"sobjects", "sobjects/Project__c/describe"} {
		r := httptest.NewRequest("GET", apiBase+path, nil)
		r.Header.Set("Authorization", "Bearer token-"+p.Tenant)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Project__c") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
