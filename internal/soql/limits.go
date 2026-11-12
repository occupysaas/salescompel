package soql

import "salescompel/internal/contract"

// Structural query limits. spec: soql-06
const (
	maxQueryChars   = 100_000
	maxWhereChars   = 4_000
	maxPathLevels   = 5 // relationship hops in one field path
	maxSubqueries   = 20
	maxSemiJoins    = 2
	maxJoinedObject = 35
	maxInList       = 1_000
	maxOrderKeys    = 32
	maxOffset       = 2_000
	maxFieldsLimit  = 200
)

func tooComplicated(msg string) error {
	return contract.Invalid("QUERY_TOO_COMPLICATED", msg)
}

// checkTextLimits enforces source and WHERE clause length recorded by Parse.
// spec: soql-06
func checkTextLimits(q *Query) error {
	if q.srcLen > maxQueryChars {
		return tooComplicated("query is too complicated")
	}
	if err := walkWhereLen(q); err != nil {
		return err
	}
	return nil
}

func walkWhereLen(q *Query) error {
	if q == nil {
		return nil
	}
	if q.whereLen > maxWhereChars {
		return tooComplicated("WHERE clause is too complicated")
	}
	for _, item := range q.Select {
		if sub, ok := item.(*SubQuery); ok {
			if err := walkWhereLen(sub.Query); err != nil {
				return err
			}
		}
	}
	if err := walkSemiSubs(q.Where, walkWhereLen); err != nil {
		return err
	}
	return walkSemiSubs(q.Having, walkWhereLen)
}

// walkSemiSubs runs fn on every semi-join subquery under e.
func walkSemiSubs(e Expr, fn func(*Query) error) error {
	switch t := e.(type) {
	case *And:
		for _, it := range t.Items {
			if err := walkSemiSubs(it, fn); err != nil {
				return err
			}
		}
	case *Or:
		for _, it := range t.Items {
			if err := walkSemiSubs(it, fn); err != nil {
				return err
			}
		}
	case *Not:
		return walkSemiSubs(t.Expr, fn)
	case *In:
		if t.Sub != nil {
			return fn(t.Sub)
		}
	}
	return nil
}

// structuralCounts tallies child subqueries, semi-joins and IN list sizes
// before name resolution.
// spec: soql-06
func structuralCounts(q *Query) (subs, semis int, err error) {
	if q == nil {
		return 0, 0, nil
	}
	subs, semis = 0, 0
	for _, item := range q.Select {
		if sub, ok := item.(*SubQuery); ok {
			subs++
			s, m, e := structuralCounts(sub.Query)
			if e != nil {
				return 0, 0, e
			}
			subs, semis = subs+s, semis+m
		}
	}
	s, m, e := exprCounts(q.Where)
	if e != nil {
		return 0, 0, e
	}
	subs, semis = subs+s, semis+m
	s, m, e = exprCounts(q.Having)
	if e != nil {
		return 0, 0, e
	}
	subs, semis = subs+s, semis+m
	return subs, semis, nil
}

func exprCounts(e Expr) (subs, semis int, err error) {
	switch t := e.(type) {
	case *And:
		for _, it := range t.Items {
			s, m, er := exprCounts(it)
			if er != nil {
				return 0, 0, er
			}
			subs, semis = subs+s, semis+m
		}
	case *Or:
		for _, it := range t.Items {
			s, m, er := exprCounts(it)
			if er != nil {
				return 0, 0, er
			}
			subs, semis = subs+s, semis+m
		}
	case *Not:
		return exprCounts(t.Expr)
	case *In:
		if len(t.Values) > maxInList {
			return 0, 0, tooComplicated("IN list is too long")
		}
		if t.Sub != nil {
			s, m, er := structuralCounts(t.Sub)
			if er != nil {
				return 0, 0, er
			}
			return 1 + s, 1 + m, nil
		}
	case *Includes:
		if len(t.Values) > maxInList {
			return 0, 0, tooComplicated("INCLUDES list is too long")
		}
	}
	return subs, semis, nil
}

// checkStructure applies the binder's structural caps on a parsed query.
// spec: soql-06
func checkStructure(q *Query) error {
	if err := checkTextLimits(q); err != nil {
		return err
	}
	subs, semis, err := structuralCounts(q)
	if err != nil {
		return err
	}
	if subs > maxSubqueries {
		return tooComplicated("too many subqueries")
	}
	if semis > maxSemiJoins {
		return tooComplicated("too many semi-joins")
	}
	return nil
}
