package platform

import (
	"net/url"
)

// queryColumnsRequested implements the bounded candidate option contract. It
// deliberately rejects ambiguous values instead of silently picking the first.
func queryColumnsRequested(values url.Values) (bool, error) {
	columns, present := values["columns"]
	if !present {
		return false, nil
	}
	if len(columns) == 1 {
		switch columns[0] {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, invalid("INVALID_QUERY_PARAMETER", "columns must be specified once as true or false")
}
