package platform

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Composite references: @{referenceId.step...}. A step is .name (a JSON
// object property, case-sensitive) or [n] (a zero-based array index). The
// first step may be an index. spec: cmp-04

const maxRefIndex = 100_000

type refStep struct {
	name  string
	index int
	isIdx bool
}

type reference struct {
	id    string
	steps []refStep
}

// tmplPart is either literal text (ref == nil) or one reference.
type tmplPart struct {
	text string
	ref  *reference
}

// refTemplate is a string split into literal text and references.
type refTemplate []tmplPart

func (t refTemplate) hasRefs() bool {
	for _, p := range t {
		if p.ref != nil {
			return true
		}
	}
	return false
}

// single reports the reference when the refTemplate is exactly one reference
// with no surrounding text.
func (t refTemplate) single() (*reference, bool) {
	if len(t) == 1 && t[0].ref != nil {
		return t[0].ref, true
	}
	return nil, false
}

func (t refTemplate) refs() []*reference {
	var out []*reference
	for _, p := range t {
		if p.ref != nil {
			out = append(out, p.ref)
		}
	}
	return out
}

// parseTemplate splits s into literal text and references. Any "@{" that
// does not start a well-formed reference is an error.
func parseTemplate(s string) (refTemplate, bool) {
	var out refTemplate
	for {
		i := strings.Index(s, "@{")
		if i < 0 {
			if s != "" {
				out = append(out, tmplPart{text: s})
			}
			return out, true
		}
		if i > 0 {
			out = append(out, tmplPart{text: s[:i]})
		}
		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			return nil, false
		}
		ref, ok := parseReference(s[i+2 : i+end])
		if !ok {
			return nil, false
		}
		out = append(out, tmplPart{ref: ref})
		s = s[i+end+1:]
	}
}

func parseReference(body string) (*reference, bool) {
	n := 0
	for n < len(body) && isRefIDChar(body[n]) {
		n++
	}
	id := body[:n]
	if !compositeRefID.MatchString(id) {
		return nil, false
	}
	ref := &reference{id: id}
	rest := body[n:]
	for rest != "" {
		switch rest[0] {
		case '.':
			k := 1
			for k < len(rest) && isRefIDChar(rest[k]) {
				k++
			}
			if k == 1 {
				return nil, false
			}
			ref.steps = append(ref.steps, refStep{name: rest[1:k]})
			rest = rest[k:]
		case '[':
			close := strings.IndexByte(rest, ']')
			if close < 2 {
				return nil, false
			}
			digits := rest[1:close]
			if len(digits) > 1 && digits[0] == '0' {
				return nil, false
			}
			idx, err := strconv.Atoi(digits)
			if err != nil || idx < 0 || idx > maxRefIndex || strings.TrimLeft(digits, "0123456789") != "" {
				return nil, false
			}
			ref.steps = append(ref.steps, refStep{index: idx, isIdx: true})
			rest = rest[close+1:]
		default:
			return nil, false
		}
	}
	return ref, len(ref.steps) > 0
}

func isRefIDChar(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// resolve walks ref's steps through the earlier subrequest bodies. A present
// key whose value is null resolves to nil with ok true.
func resolve(ref *reference, bodies map[string]any) (any, bool) {
	cur, ok := bodies[ref.id]
	if !ok {
		return nil, false
	}
	for _, st := range ref.steps {
		if st.isIdx {
			list, isList := cur.([]any)
			if !isList || st.index >= len(list) {
				return nil, false
			}
			cur = list[st.index]
			continue
		}
		obj, isObj := cur.(map[string]any)
		if !isObj {
			return nil, false
		}
		next, present := obj[st.name]
		if !present {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// scalarText renders a resolved scalar for embedding in a longer string.
func scalarText(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "null", true
	case string:
		return x, true
	case bool:
		return strconv.FormatBool(x), true
	case json.Number:
		return x.String(), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	}
	return "", false
}

func isScalar(v any) bool {
	_, ok := scalarText(v)
	return ok
}

// renderText substitutes every reference in t as text.
func renderText(t refTemplate, bodies map[string]any) (string, bool) {
	var b strings.Builder
	for _, p := range t {
		if p.ref == nil {
			b.WriteString(p.text)
			continue
		}
		v, ok := resolve(p.ref, bodies)
		if !ok {
			return "", false
		}
		s, ok := scalarText(v)
		if !ok {
			return "", false
		}
		b.WriteString(s)
	}
	return b.String(), true
}

// substituteBody returns a copy of v with every reference in string values
// resolved. A string that is exactly one reference takes the referenced
// scalar with its JSON type. ok is false when any reference cannot resolve.
func substituteBody(v any, bodies map[string]any) (any, bool) {
	switch x := v.(type) {
	case string:
		t, parsed := parseTemplate(x)
		if !parsed || !t.hasRefs() {
			return x, parsed
		}
		if ref, one := t.single(); one {
			val, ok := resolve(ref, bodies)
			if !ok || !isScalar(val) {
				return nil, false
			}
			return val, true
		}
		return renderText(t, bodies)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			sub, ok := substituteBody(item, bodies)
			if !ok {
				return nil, false
			}
			out[k] = sub
		}
		return out, true
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			sub, ok := substituteBody(item, bodies)
			if !ok {
				return nil, false
			}
			out[i] = sub
		}
		return out, true
	}
	return v, true
}

// bodyRefs lists every reference in v's string values, or false when any
// "@{" is malformed or appears in a property name.
func bodyRefs(v any) ([]*reference, bool) {
	switch x := v.(type) {
	case string:
		t, ok := parseTemplate(x)
		return t.refs(), ok
	case map[string]any:
		var out []*reference
		for k, item := range x {
			if strings.Contains(k, "@{") {
				return nil, false
			}
			refs, ok := bodyRefs(item)
			if !ok {
				return nil, false
			}
			out = append(out, refs...)
		}
		return out, true
	case []any:
		var out []*reference
		for _, item := range x {
			refs, ok := bodyRefs(item)
			if !ok {
				return nil, false
			}
			out = append(out, refs...)
		}
		return out, true
	}
	return nil, true
}
