package fieldtype

import "strings"

// MultiPicklist canonicalizes a multi-select value: split on ';', trim
// spaces, drop empty entries and duplicates, order by definition then input.
func MultiPicklist(input string, defined []string) (string, []string) {
	seen := map[string]bool{}
	selected := []string{}
	for _, part := range strings.Split(input, ";") {
		part = strings.Trim(part, " ")
		if part != "" && !seen[part] {
			seen[part] = true
			selected = append(selected, part)
		}
	}
	out := []string{}
	listed := map[string]bool{}
	for _, d := range defined {
		listed[d] = true
		if seen[d] {
			out = append(out, d)
		}
	}
	for _, s := range selected {
		if !listed[s] {
			out = append(out, s)
		}
	}
	return strings.Join(out, ";"), out
}
