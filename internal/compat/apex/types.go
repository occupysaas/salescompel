// Package apex implements the isolated Plan 19 synthetic feasibility experiment.
// It is not a production Apex runtime or a Salesforce compatibility claim.
package apex

import "fmt"

type Identity struct{ Tenant, Principal string }
type Policy struct {
	Identity                       Identity
	QueryAllowed, NameWriteAllowed bool
	DeniedWriteIDs                 []string
}
type Diagnostic struct {
	Code         string
	Line, Column int
}

func (d *Diagnostic) Error() string { return fmt.Sprintf("%s at %d:%d", d.Code, d.Line, d.Column) }
func diagnostic(code string) error  { return &Diagnostic{Code: code} }

type valueKind uint8

const (
	nullKind valueKind = iota
	integerKind
	booleanKind
	stringKind
)

// Value is an immutable tagged scalar; its zero value is Null.
type Value struct {
	kind valueKind
	i    int32
	b    bool
	s    string
}

func Integer(v int32) Value          { return Value{kind: integerKind, i: v} }
func Boolean(v bool) Value           { return Value{kind: booleanKind, b: v} }
func String(v string) Value          { return Value{kind: stringKind, s: v} }
func Null() Value                    { return Value{} }
func (v Value) Int() (int32, bool)   { return v.i, v.kind == integerKind }
func (v Value) Bool() (bool, bool)   { return v.b, v.kind == booleanKind }
func (v Value) Text() (string, bool) { return v.s, v.kind == stringKind }
func (v Value) IsNull() bool         { return v.kind == nullKind }
