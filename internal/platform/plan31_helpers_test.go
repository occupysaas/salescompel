package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// apiCall sends one request through h and decodes a JSON response body.
func apiCall(t *testing.T, h http.Handler, method, path, body, token string) (int, any, string) {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var result any
	if w.Body.Len() > 0 {
		if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
			t.Fatalf("non-json response %d %s", w.Code, w.Body.String())
		}
	}
	return w.Code, result, w.Body.String()
}

func errorCode(v any) string {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return ""
	}
	m, _ := list[0].(map[string]any)
	code, _ := m["errorCode"].(string)
	return code
}
