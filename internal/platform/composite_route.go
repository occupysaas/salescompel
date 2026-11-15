package platform

import (
	"net/url"
	"regexp"
	"strings"
)

// Subrequest routing. URLs are parsed into segments and query values before
// any reference is substituted, so a referenced value is always data inside
// one segment or one value and can never change the route. spec: cmp-03, cmp-04

type routeKind int

const (
	routeUnsupported routeKind = iota // item error 404
	routeWrongMethod                  // item error 405
	routeCreate
	routeRead
	routeUpdate
	routeDelete
	routeUpsert
	routeQuery
	routeQueryMore
	routeCollCreate
	routeCollUpdate
	routeCollUpsert
	routeCollDelete
	routeCollRetrieveGet
	routeCollRetrievePost
)

var compositeName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,79}$`)

const compositeVersionPrefix = "/services/data/"

// subRoute is a parsed subrequest URL.
type subRoute struct {
	kind   routeKind
	object string                 // object API name from the URL
	value  refTemplate            // record ID, external ID value or locator
	query  map[string]refTemplate // decoded query parameter values
}

func (r subRoute) countsTowardQueryLimit() bool {
	return r.kind == routeQuery || r.kind == routeQueryMore || r.isCollections()
}

func (r subRoute) isCollections() bool {
	return r.kind >= routeCollCreate && r.kind <= routeCollRetrievePost
}

// refs lists every reference in the URL.
func (r subRoute) refs() []*reference {
	out := r.value.refs()
	for _, v := range r.query {
		out = append(out, v.refs()...)
	}
	return out
}

// parseSubrequestURL checks URL syntax (a request error when malformed) and
// classifies the route (unsupported routes are item errors, reported later).
func parseSubrequestURL(method, raw string) (subRoute, bool) {
	if !strings.HasPrefix(raw, compositeVersionPrefix) || strings.ContainsAny(raw, "#\\") {
		return subRoute{}, false
	}
	path, rawQuery, hasQuery := strings.Cut(raw[len(compositeVersionPrefix):], "?")
	if hasQuery && rawQuery == "" {
		return subRoute{}, false
	}
	// Like the standalone handler, one trailing slash is ignored.
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	segs := strings.Split(path, "/")
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return subRoute{}, false
		}
	}
	query, ok := parseSubrequestQuery(rawQuery)
	if !ok {
		return subRoute{}, false
	}
	route := subRoute{kind: routeUnsupported, query: query}
	if segs[0] != "v65.0" {
		return route, literalSegments(segs)
	}
	segs = segs[1:]
	// Only these positions may hold a reference or a percent escape.
	var valueSeg = -1
	switch {
	case len(segs) == 3 && segs[0] == "sobjects":
		valueSeg = 2
	case len(segs) == 4 && segs[0] == "sobjects":
		valueSeg = 3
	case len(segs) == 2 && segs[0] == "query":
		valueSeg = 1
	}
	unsupported := false
	for i, s := range segs {
		if i == valueSeg {
			continue
		}
		if strings.ContainsAny(s, "%@{}") {
			return subRoute{}, false
		}
		unsupported = unsupported || !compositeName.MatchString(s)
	}
	if valueSeg >= 0 {
		t, ok := parseTemplate(segs[valueSeg])
		// A query locator is literal: the spec allows references only in
		// record-ID and external-ID value segments.
		if !ok || (t.hasRefs() && (len(t) != 1 || segs[0] == "query")) {
			return subRoute{}, false
		}
		if !t.hasRefs() {
			decoded, err := url.PathUnescape(segs[valueSeg])
			if err != nil || decoded == "" {
				return subRoute{}, false
			}
			t = refTemplate{{text: decoded}}
		}
		route.value = t
	}
	route.kind = classify(method, segs, query)
	if unsupported {
		route.kind = routeUnsupported
	}
	if len(segs) >= 2 && segs[0] == "sobjects" {
		route.object = segs[1]
	}
	if len(segs) >= 3 && segs[0] == "composite" && segs[1] == "sobjects" {
		route.object = segs[2]
	}
	return route, true
}

// literalSegments accepts a URL for another API version only when it has no
// references or escapes, so it can be reported as an item error.
func literalSegments(segs []string) bool {
	for _, s := range segs {
		if strings.ContainsAny(s, "%@{}") {
			return false
		}
	}
	return true
}

func classify(method string, segs []string, query map[string]refTemplate) routeKind {
	allowed := func(names ...string) bool {
		for k := range query {
			found := false
			for _, n := range names {
				found = found || k == n
			}
			if !found {
				return false
			}
		}
		return true
	}
	pick := func(byMethod map[string]routeKind, params ...string) routeKind {
		kind, ok := byMethod[method]
		if !ok {
			return routeWrongMethod
		}
		if !allowed(params...) {
			return routeUnsupported
		}
		return kind
	}
	switch {
	case len(segs) == 2 && segs[0] == "sobjects":
		return pick(map[string]routeKind{"POST": routeCreate})
	case len(segs) == 3 && segs[0] == "sobjects":
		if method == "GET" {
			return pick(map[string]routeKind{"GET": routeRead}, "fields")
		}
		return pick(map[string]routeKind{"PATCH": routeUpdate, "DELETE": routeDelete})
	case len(segs) == 4 && segs[0] == "sobjects" && strings.EqualFold(segs[1], "Account") && strings.EqualFold(segs[2], "Compat_Key__c"):
		return pick(map[string]routeKind{"PATCH": routeUpsert})
	case len(segs) == 1 && segs[0] == "query":
		if _, ok := query["q"]; !ok && method == "GET" {
			return routeUnsupported
		}
		return pick(map[string]routeKind{"GET": routeQuery}, "q", "columns")
	case len(segs) == 2 && segs[0] == "query":
		return pick(map[string]routeKind{"GET": routeQueryMore})
	case len(segs) == 2 && segs[0] == "composite" && segs[1] == "sobjects":
		if method == "DELETE" {
			return pick(map[string]routeKind{"DELETE": routeCollDelete}, "ids", "allOrNone")
		}
		return pick(map[string]routeKind{"POST": routeCollCreate, "PATCH": routeCollUpdate})
	case len(segs) == 3 && segs[0] == "composite" && segs[1] == "sobjects":
		if method == "GET" {
			return pick(map[string]routeKind{"GET": routeCollRetrieveGet}, "ids", "fields")
		}
		return pick(map[string]routeKind{"POST": routeCollRetrievePost})
	case len(segs) == 4 && segs[0] == "composite" && segs[1] == "sobjects" && strings.EqualFold(segs[2], "Account") && strings.EqualFold(segs[3], "Compat_Key__c"):
		return pick(map[string]routeKind{"PATCH": routeCollUpsert})
	}
	return routeUnsupported
}

// parseSubrequestQuery splits a raw query into decoded values that may hold
// references. Keys must be literal and unique.
func parseSubrequestQuery(raw string) (map[string]refTemplate, bool) {
	out := map[string]refTemplate{}
	if raw == "" {
		return out, true
	}
	for _, pair := range strings.Split(raw, "&") {
		k, v, _ := strings.Cut(pair, "=")
		key, err := url.QueryUnescape(k)
		if err != nil || key == "" || strings.Contains(key, "@{") {
			return nil, false
		}
		if _, dup := out[key]; dup {
			return nil, false
		}
		t, ok := parseTemplate(v)
		if !ok {
			return nil, false
		}
		for i := range t {
			if t[i].ref == nil {
				decoded, err := url.QueryUnescape(t[i].text)
				if err != nil {
					return nil, false
				}
				t[i].text = decoded
			}
		}
		if t == nil {
			t = refTemplate{}
		}
		out[key] = t
	}
	return out, true
}
