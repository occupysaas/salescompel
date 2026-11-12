package soql

import (
	"time"

	"salescompel/internal/metadata"
)

// Clock reports the query's evaluation instant. Tests inject a fixed clock.
type Clock func() time.Time

// Param is a typed literal carried as a SQL parameter, never as query text.
// Value is a string, bool or UTC time.Time; a nil Value is null. Cast names
// the SQL type the compiler should use (text, numeric, bool, date, timestamptz).
// spec: soql-01
type Param struct {
	Value any
	Cast  string
}

// PathStep is one parent-relationship hop: the object reached and the
// reference field taken from the previous level to reach it.
type PathStep struct {
	Object metadata.Object
	Via    metadata.Field
}

// BoundValue is a typed scalar: a field path, an aggregate or a function
// call. Key is the result key (select key, alias) when set.
type BoundValue struct {
	Path  []PathStep
	Field metadata.Field
	Agg   string // COUNT, COUNT_DISTINCT, SUM, AVG, MIN, MAX
	Func  string // toLabel or a documented date function
	Label bool   // Func is toLabel
	Key   string
}

// BoundColumn is one SELECT item with its result key.
type BoundColumn struct {
	Key   string
	Path  []PathStep
	Field metadata.Field
	Agg   string
	Label bool
	Func  string
}

// BoundOrder is one ORDER BY key.
type BoundOrder struct {
	Value BoundValue
	Desc  bool
	Nulls NullPlacement
}

// BoundChild is one child subquery. Task 6 compiles it; this task resolves
// the relationship name and binds the inner query.
type BoundChild struct {
	Key          string
	Relationship metadata.ChildRelationship
	Query        *Bound
}

// Bound is a query resolved against the catalog, carrying typed parameters
// and catalog entries only — no user text.
type Bound struct {
	Object    metadata.Object
	Columns   []BoundColumn
	Children  []BoundChild
	Where     BoundExpr
	GroupBy   []BoundValue
	Having    BoundExpr
	Order     []BoundOrder
	Limit     *int
	Offset    *int
	Aggregate bool
	CountOnly bool
}

// BoundExpr is the typed boolean tree of a WHERE or HAVING clause.
type BoundExpr interface{ boundExpr() }

type BoundAnd struct{ Items []BoundExpr }
type BoundOr struct{ Items []BoundExpr }
type BoundNot struct{ Expr BoundExpr }

// BoundCmp compares a typed left operand with a typed parameter. A nil
// Param.Value is the null test: "=" means IS NULL and "!=" means IS NOT NULL.
// Date-literal comparisons are rewritten into these at bind time.
type BoundCmp struct {
	Left  BoundValue
	Op    string // =, !=, <, <=, >, >=, LIKE
	Right Param
}

// BoundIn is IN/NOT IN over a typed list, or a semi-/anti-join subquery.
type BoundIn struct {
	Left   BoundValue
	Not    bool
	Values []Param
	Semi   *Bound // set instead of Values for a semi-/anti-join
}

// BoundIncludes is INCLUDES/EXCLUDES on a multi-select picklist. Values are
// raw semicolon elements; the compiler splits and case-folds them.
type BoundIncludes struct {
	Left    BoundValue
	Exclude bool
	Values  []Param
}

func (*BoundAnd) boundExpr()      {}
func (*BoundOr) boundExpr()       {}
func (*BoundNot) boundExpr()      {}
func (*BoundCmp) boundExpr()      {}
func (*BoundIn) boundExpr()       {}
func (*BoundIncludes) boundExpr() {}
