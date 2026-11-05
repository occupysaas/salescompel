package platform

import "salescompel/internal/query"

type Query = query.Query
type NullPlacement = query.NullPlacement

const (
	NullDefault = query.NullDefault
	NullFirst   = query.NullFirst
	NullLast    = query.NullLast
)

var ParseQuery = query.Parse
