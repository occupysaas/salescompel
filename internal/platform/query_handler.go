package platform

import (
	"context"
	"net/http"
	"net/url"
)

// restQuery runs SOQL text exactly as GET /query does, including the
// Organization and currency discovery answers, and returns the REST body.
// options carries the request's query parameters (q is ignored here).
func (s *Store) restQuery(ctx context.Context, p Principal, qtext string, options url.Values, batch int) (map[string]any, error) {
	if isOrganizationDiscovery(qtext) {
		return map[string]any{"totalSize": 1, "done": true, "records": []any{map[string]any{"Name": p.Tenant, "InstanceName": nil, "IsSandbox": false, "TrialExpirationDate": nil, "NamespacePrefix": nil, "OrganizationType": "SalesCompel"}}}, nil
	}
	if isCurrencyDiscovery(qtext) {
		return map[string]any{"totalSize": 1, "done": true, "records": []any{map[string]any{"attributes": map[string]any{"type": "Organization"}, "DefaultCurrencyIsoCode": p.Catalog.Currency}}}, nil
	}
	columns, e := queryColumnsRequested(options)
	if e != nil {
		return nil, e
	}
	if columns {
		res, md, e := s.QueryColumns(ctx, p, qtext, batch)
		if e != nil {
			return nil, e
		}
		result := map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records, "columnMetadata": md}
		if res.NextLocator != "" {
			result["nextRecordsUrl"] = apiBase + "query/" + res.NextLocator
		}
		return result, nil
	}
	res, e := s.Query(ctx, p, qtext, batch)
	if e != nil {
		return nil, e
	}
	result := map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records}
	if res.NextLocator != "" {
		result["nextRecordsUrl"] = apiBase + "query/" + res.NextLocator
	}
	return result, nil
}

// restQueryMore serves one locator page in the REST body shape.
func (s *Store) restQueryMore(ctx context.Context, p Principal, locator string, batch int) (map[string]any, error) {
	res, e := s.QueryMore(ctx, p, locator, batch)
	if e != nil {
		return nil, e
	}
	result := map[string]any{"totalSize": res.TotalSize, "done": res.Done, "records": res.Records}
	if res.NextLocator != "" {
		result["nextRecordsUrl"] = apiBase + "query/" + res.NextLocator
	}
	return result, nil
}

// serveQuery handles /query and /query/{locator}. parts[0] is "query".
func serveQuery(s *Store, ctx context.Context, p Principal, w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 1 {
		if !method(w, r, "GET") {
			return
		}
		qtext := r.URL.Query().Get("q")
		var options url.Values
		if !isOrganizationDiscovery(qtext) && !isCurrencyDiscovery(qtext) {
			var optionErr error
			if options, optionErr = url.ParseQuery(r.URL.RawQuery); optionErr != nil {
				writeError(w, invalid("INVALID_QUERY_PARAMETER", "Invalid query parameter encoding"))
				return
			}
		}
		result, e := s.restQuery(ctx, p, qtext, options, queryBatchSize(r))
		if e != nil {
			writeError(w, e)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if len(parts) != 2 {
		writeError(w, invalid("INVALID_QUERY_LOCATOR", "Query locator is invalid"))
		return
	}
	if !method(w, r, "GET") {
		return
	}
	result, e := s.restQueryMore(ctx, p, parts[1], queryBatchSize(r))
	if e != nil {
		writeError(w, e)
		return
	}
	writeJSON(w, 200, result)
}
