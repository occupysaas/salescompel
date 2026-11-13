package platform

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"salescompel/internal/metadata"
	"salescompel/internal/records"
	"strings"
	"unicode/utf8"

	apicontract "salescompel/api"
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
	data := map[string]any{}
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return nil, invalid("JSON_PARSER_ERROR", "Expected a JSON object")
	}
	// Preserve member order until case-insensitive duplicates have collapsed.
	keys := map[string]string{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, invalid("JSON_PARSER_ERROR", "Invalid JSON member")
		}
		key, ok := token.(string)
		if !ok {
			return nil, invalid("JSON_PARSER_ERROR", "Invalid JSON member")
		}
		var value any
		if err := dec.Decode(&value); err != nil {
			return nil, invalid("JSON_PARSER_ERROR", "Invalid JSON value")
		}
		folded := strings.ToLower(key)
		// EqualFold also covers Unicode simple-fold variants.
		for previous := range keys {
			if strings.EqualFold(previous, key) {
				folded = previous
				break
			}
		}
		if previous, ok := keys[folded]; ok {
			delete(data, previous)
		}
		keys[folded] = key
		data[key] = value
	}
	if end, err := dec.Token(); err != nil || end != json.Delim('}') {
		return nil, invalid("JSON_PARSER_ERROR", "Invalid JSON object")
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
	return NewHandlerWithOptions(s, assetDir, Options{})
}
func NewHandlerWithOptions(s *Store, assetDir string, o Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CLI joins its trailing-slash instance URL with absolute service paths.
		// Preserve RawPath so encoded slashes inside external IDs remain data.
		requestPath := "/" + strings.TrimLeft(r.URL.Path, "/")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		if o.PublicURL != "" {
			var oauth func(*Store, Options, http.ResponseWriter, *http.Request)
			switch requestPath {
			case "/.well-known/openid-configuration":
				oauth = handleDiscovery
			case "/services/oauth2/authorize":
				oauth = handleAuthorize
			case "/services/oauth2/authorize.css":
				oauth = handleAuthorizeCSS
			case "/services/oauth2/token":
				oauth = handleToken
			case "/services/oauth2/revoke":
				oauth = handleRevoke
			case "/services/oauth2/introspect":
				oauth = handleIntrospect
			}
			if oauth != nil {
				ctx, cancel := timeBound(r)
				defer cancel()
				r = r.WithContext(ctx)
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
				oauth(s, o, w, r)
				return
			}
		}
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
		if !strings.HasPrefix(requestPath, "/services/") && !(o.PublicURL != "" && strings.HasPrefix(requestPath, "/id/")) {
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
		p, err := s.AuthenticateAt(ctx, token, o.now())
		if err != nil {
			writeError(w, err)
			return
		}
		path := strings.TrimSuffix("/"+strings.TrimLeft(r.URL.EscapedPath(), "/"), "/")
		class := routeData
		if path == "/services/oauth2/userinfo" || strings.HasPrefix(path, "/id/") {
			class = routeIdentity
		}
		if !scopeAllows(p, class) {
			writeError(w, problem(403, "INSUFFICIENT_ACCESS", "Insufficient access"))
			return
		}
		if strings.HasPrefix(path, "/id/") {
			handleIdentity(p, o, w, r)
			return
		}
		if path == apicontract.CandidateOpenAPIPath {
			if method(w, r, "GET") {
				w.Header().Set("Content-Type", apicontract.CandidateOpenAPIContentType)
				w.WriteHeader(http.StatusOK)
				if _, wErr := io.WriteString(w, apicontract.CandidateOpenAPI()); wErr != nil {
					return
				}
			}
			return
		}
		if path == "/services/data" {
			if method(w, r, "GET") {
				writeJSON(w, 200, []any{map[string]any{"version": "65.0", "label": "SalesCompel candidate", "url": strings.TrimSuffix(apiBase, "/")}})
			}
			return
		}
		if path == "/services/oauth2/userinfo" {
			if method(w, r, "GET") {
				info := map[string]any{"user_id": p.User, "organization_id": p.Tenant, "preferred_username": p.User, "username": p.User}
				if p.Username != "" {
					info["organization_id"] = orgID(p.Tenant)
					info["username"], info["preferred_username"] = p.Username, p.Username
					info["sub"] = identityURL(p, o)
				}
				writeJSON(w, 200, info)
			}
			return
		}
		// The pinned CLI uses v51.0 for this discovery call only.
		if path == "/services/data/v51.0/sobjects/User/"+p.User || path == apiBase+"sobjects/User/"+p.User {
			if method(w, r, "GET") {
				username := p.User
				if p.Username != "" {
					username = p.Username
				}
				writeJSON(w, 200, map[string]any{"Id": p.User, "Username": username, "Name": p.User, "LanguageLocaleKey": "en_US", "LocaleSidKey": "en_US", "TimeZoneSidKey": "UTC", "Email": p.User + "@example.invalid"})
			}
			return
		}
		if path == strings.TrimSuffix(apiBase, "/") {
			if method(w, r, "GET") {
				writeJSON(w, 200, map[string]string{"sobjects": apiBase + "sobjects", "query": apiBase + "query", "openapi": apicontract.CandidateOpenAPIPath, "composite": apiBase + "composite"})
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
		catalog, e := s.Catalog(ctx, p)
		if e != nil {
			writeError(w, e)
			return
		}
		p.Catalog = &catalog
		if parts[0] == "composite" && len(parts) == 1 {
			if !method(w, r, "POST") {
				return
			}
			req, e := decodeComposite(r.Body)
			if e != nil {
				writeError(w, e)
				return
			}
			result, e := s.Composite(ctx, p, req)
			if e != nil {
				writeError(w, e)
				return
			}
			writeJSON(w, 200, result)
			return
		}
		if parts[0] == "query" && len(parts) == 1 {
			if !method(w, r, "GET") {
				return
			}
			qtext := r.URL.Query().Get("q")
			if isOrganizationDiscovery(qtext) {
				writeJSON(w, 200, map[string]any{"totalSize": 1, "done": true, "records": []any{map[string]any{"Name": p.Tenant, "InstanceName": nil, "IsSandbox": false, "TrialExpirationDate": nil, "NamespacePrefix": nil, "OrganizationType": "SalesCompel"}}})
				return
			}
			if isCurrencyDiscovery(qtext) {
				writeJSON(w, 200, map[string]any{"totalSize": 1, "done": true, "records": []any{map[string]any{"attributes": map[string]any{"type": "Organization"}, "DefaultCurrencyIsoCode": catalog.Currency}}})
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
			batch := queryBatchSize(r)
			if columns {
				res, md, e := s.QueryColumns(ctx, p, qtext, batch)
				if e != nil {
					writeError(w, e)
					return
				}
				result := map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records, "columnMetadata": md}
				if res.NextLocator != "" {
					result["nextRecordsUrl"] = apiBase + "query/" + res.NextLocator
				}
				writeJSON(w, 200, result)
				return
			}
			res, e := s.Query(ctx, p, qtext, batch)
			if e != nil {
				writeError(w, e)
				return
			}
			result := map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records}
			if res.NextLocator != "" {
				result["nextRecordsUrl"] = apiBase + "query/" + res.NextLocator
			}
			writeJSON(w, 200, result)
			return
		}
		if parts[0] == "query" {
			if len(parts) != 2 {
				writeError(w, invalid("INVALID_QUERY_LOCATOR", "Query locator is invalid"))
				return
			}
			if !method(w, r, "GET") {
				return
			}
			res, e := s.QueryMore(ctx, p, parts[1], queryBatchSize(r))
			if e != nil {
				writeError(w, e)
				return
			}
			result := map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records}
			if res.NextLocator != "" {
				result["nextRecordsUrl"] = apiBase + "query/" + res.NextLocator
			}
			writeJSON(w, 200, result)
			return
		}
		if parts[0] == "candidate" && len(parts) == 4 && parts[1] == "sobjects" && parts[3] == "storedFields" {
			// Candidate-only: stored-not-enforced definitions are never described.
			if !method(w, r, "GET") {
				return
			}
			definition, lookupErr := catalog.Object(parts[2])
			if lookupErr != nil {
				writeError(w, problem(404, "NOT_FOUND", "Object not found"))
				return
			}
			stored := []metadata.StoredField{}
			for _, sf := range catalog.Stored {
				if sf.Object == definition.Name {
					stored = append(stored, sf)
				}
			}
			writeJSON(w, 200, map[string]any{"storedFields": stored})
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
			for _, object := range catalog.Objects {
				d, dErr := records.Describe(p, object.Name)
				if dErr != nil {
					writeError(w, problem(500, "INTERNAL_ERROR", "Describe failed"))
					return
				}
				objects = append(objects, d)
			}
			writeJSON(w, 200, map[string]any{"sobjects": objects, "encoding": "UTF-8", "maxBatchSize": 1})
			return
		}
		definition, lookupErr := catalog.Object(parts[1])
		object, known := definition.Name, lookupErr == nil
		if !known {
			writeError(w, problem(404, "NOT_FOUND", "Object not found"))
			return
		}
		if len(parts) == 3 && parts[2] == "describe" {
			if method(w, r, "GET") {
				d, dErr := records.Describe(p, object)
				if dErr != nil {
					writeError(w, problem(500, "INTERNAL_ERROR", "Describe failed"))
					return
				}
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
		if len(parts) == 4 && object == "Account" && strings.EqualFold(parts[2], "Compat_Key__c") {
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
