package platform

import "salescompel/internal/query"

type Query = query.Query
type NullPlacement = query.NullPlacement

const (
	NullDefault = query.NullDefault
	NullFirst   = query.NullFirst
	NullLast    = query.NullLast
)

// ParseQuery is a deprecated wrapper kept for existing unit tests until Task 9.
// Deprecated: use Store.Query which parses SOQL text directly.
var ParseQuery = query.Parse
