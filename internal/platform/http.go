package platform

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const apiBase = "/services/data/v65.0/"

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if status != 204 {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func writeError(w http.ResponseWriter, err error) {
	var api *APIError
	if !errors.As(err, &api) {
		api = &APIError{Status: 500, Code: "SERVER_UNAVAILABLE", Message: "Request failed"}
	}
	writeJSON(w, api.Status, []*APIError{api})
}
func body(r *http.Request) (map[string]any, error) {
	var data map[string]any
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&data); err != nil || data == nil {
		return nil, invalid("JSON_PARSER_ERROR", "Expected a JSON object")
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		return nil, invalid("JSON_PARSER_ERROR", "Unexpected trailing JSON")
	}
	return data, nil
}
func method(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, m := range allowed {
		if r.Method == m {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, problem(405, "METHOD_NOT_ALLOWED", "Unsupported method"))
	return false
}
func NewHandler(s *Store, assetDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CLI joins its trailing-slash instance URL with absolute service paths.
		// Preserve RawPath so encoded slashes inside external IDs remain data.
		requestPath := "/" + strings.TrimLeft(r.URL.Path, "/")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		if r.URL.Path == "/healthz" {
			if !method(w, r, "GET") {
				return
			}
			ctx, cancel := timeBound(r)
			defer cancel()
			if err := s.pool.Ping(ctx); err != nil {
				writeError(w, problem(503, "SERVER_UNAVAILABLE", "Database unavailable"))
				return
			}
			writeJSON(w, 200, map[string]any{"status": "ok"})
			return
		}
		if !strings.HasPrefix(requestPath, "/services/") {
			if assetDir == "" {
				http.NotFound(w, r)
				return
			}
			if !method(w, r, "GET", "HEAD") {
				return
			}
			if r.URL.Path == "/" {
				http.ServeFile(w, r, filepath.Join(assetDir, "index.html"))
				return
			}
			// Never generate directory listings of the built asset tree.
			if strings.HasSuffix(r.URL.Path, "/") {
				http.NotFound(w, r)
				return
			}
			http.FileServer(http.Dir(assetDir)).ServeHTTP(w, r)
			return
		}
		ctx, cancel := timeBound(r)
		defer cancel()
		r = r.WithContext(ctx)
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		auth := r.Header.Get("Authorization")
		token := ""
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
		p, err := s.Authenticate(ctx, token)
		if err != nil {
			writeError(w, err)
			return
		}
		path := strings.TrimSuffix("/"+strings.TrimLeft(r.URL.EscapedPath(), "/"), "/")
		if path == "/services/data" {
			if method(w, r, "GET") {
				writeJSON(w, 200, []any{map[string]any{"version": "65.0", "label": "SalesCompel candidate", "url": strings.TrimSuffix(apiBase, "/")}})
			}
			return
		}
		if path == "/services/oauth2/userinfo" {
			if method(w, r, "GET") {
				writeJSON(w, 200, map[string]any{"user_id": p.User, "organization_id": p.Tenant, "preferred_username": p.User, "username": p.User})
			}
			return
		}
		// The pinned CLI uses v51.0 for this discovery call only.
		if path == "/services/data/v51.0/sobjects/User/"+p.User || path == apiBase+"sobjects/User/"+p.User {
			if method(w, r, "GET") {
				writeJSON(w, 200, map[string]any{"Id": p.User, "Username": p.User, "Name": p.User, "LanguageLocaleKey": "en_US", "LocaleSidKey": "en_US", "TimeZoneSidKey": "UTC", "Email": p.User + "@example.invalid"})
			}
			return
		}
		if path == strings.TrimSuffix(apiBase, "/") {
			if method(w, r, "GET") {
				writeJSON(w, 200, map[string]string{"sobjects": apiBase + "sobjects", "query": apiBase + "query"})
			}
			return
		}
		if !strings.HasPrefix(path, apiBase) {
			writeError(w, problem(404, "NOT_FOUND", "Unsupported API version or resource"))
			return
		}
		parts := strings.Split(strings.TrimPrefix(path, apiBase), "/")
		for i, part := range parts {
			decoded, decodeErr := url.PathUnescape(part)
			if decodeErr != nil || !utf8.ValidString(decoded) || strings.ContainsRune(decoded, 0) {
				writeError(w, invalid("INVALID_FIELD", "Invalid URL encoding"))
				return
			}
			parts[i] = decoded
		}
		if parts[0] == "query" && len(parts) == 1 {
			if !method(w, r, "GET") {
				return
			}
			q, e := ParseQuery(r.URL.Query().Get("q"))
			if e != nil {
				writeError(w, e)
				return
			}
			options, optionErr := url.ParseQuery(r.URL.RawQuery)
			if optionErr != nil {
				writeError(w, invalid("INVALID_QUERY_PARAMETER", "Invalid query parameter encoding"))
				return
			}
			columns, e := queryColumnsRequested(options)
			if e != nil {
				writeError(w, e)
				return
			}
			rows, e := s.Select(ctx, p, q)
			if e != nil {
				writeError(w, e)
				return
			}
			result := map[string]any{"totalSize": len(rows), "done": true, "records": rows}
			if columns {
				metadata, err := queryColumnMetadata(p, q)
				if err != nil {
					writeError(w, err)
					return
				}
				result["columnMetadata"] = metadata
			}
			writeJSON(w, 200, result)
			return
		}
		if parts[0] != "sobjects" {
			writeError(w, problem(404, "NOT_FOUND", "Resource not implemented by this candidate"))
			return
		}
		if len(parts) == 1 {
			if !method(w, r, "GET") {
				return
			}
			objects := []Object{}
			for _, name := range []string{"Account", "Contact"} {
				d, _ := Describe(p, name)
				objects = append(objects, d)
			}
			writeJSON(w, 200, map[string]any{"sobjects": objects, "encoding": "UTF-8", "maxBatchSize": 1})
			return
		}
		object := parts[1]
		if _, e := Describe(p, object); e != nil {
			writeError(w, e)
			return
		}
		if len(parts) == 3 && parts[2] == "describe" {
			if method(w, r, "GET") {
				d, _ := Describe(p, object)
				writeJSON(w, 200, d)
			}
			return
		}
		if len(parts) == 2 {
			if !method(w, r, "POST") {
				return
			}
			data, e := body(r)
			if e != nil {
				writeError(w, e)
				return
			}
			id, e := s.Create(ctx, p, object, data)
			if e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, 201, map[string]any{"id": id, "success": true, "errors": []any{}})
			return
		}
		if len(parts) == 4 && object == "Account" && parts[2] == "Compat_Key__c" {
			if !method(w, r, "PATCH") {
				return
			}
			data, e := body(r)
			if e != nil {
				writeError(w, e)
				return
			}
			id, created, e := s.Upsert(ctx, p, parts[3], data)
			if e != nil {
				writeError(w, e)
				return
			}
			status := 200
			if created {
				status = 201
			}
			writeJSON(w, status, map[string]any{"id": id, "created": created, "success": true, "errors": []any{}})
			return
		}
		if len(parts) != 3 {
			writeError(w, problem(404, "NOT_FOUND", "Resource not implemented"))
			return
		}
		if !method(w, r, "GET", "PATCH", "DELETE") {
			return
		}
		id := parts[2]
		switch r.Method {
		case "GET":
			row, e := s.Read(ctx, p, object, id)
			if e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, 200, row)
		case "PATCH":
			data, e := body(r)
			if e != nil {
				writeError(w, e)
				return
			}
			if e = s.Update(ctx, p, object, id, data); e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, 204, nil)
		case "DELETE":
			if e := s.Delete(ctx, p, object, id); e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, 204, nil)
		}
	})
}

// AssetDir validates configuration before any listener starts.
func AssetDir(dir string) error {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil || info.IsDir() {
		return errors.New("web assets missing; run the frontend build")
	}
	return nil
}
