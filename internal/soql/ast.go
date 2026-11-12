package soql

import "encoding/json"

type LiteralKind int

const (
	LitString LiteralKind = iota
	LitNumber
	LitBool
	LitNull
	LitDate
	LitDateTime
	LitDateLiteral
)

type NullPlacement uint8

const (
	NullDefault NullPlacement = iota
	NullFirst
	NullLast
)

// Query is one SELECT statement, also used for subqueries.
type Query struct {
	Select  []SelectItem `json:"select"`
	From    Name         `json:"from"`
	Alias   string       `json:"alias,omitempty"`
	Where   Expr         `json:"where"`
	GroupBy []ValueExpr  `json:"groupBy"`
	Having  Expr         `json:"having"`
	OrderBy []OrderItem  `json:"orderBy"`
	Limit   *int         `json:"limit"`
	Offset  *int         `json:"offset"`
	With    string       `json:"with,omitempty"`
	Pos     Pos          `json:"-"`
}

type OrderItem struct {
	Value ValueExpr     `json:"value"`
	Desc  bool          `json:"desc"`
	Nulls NullPlacement `json:"nulls"`
}

type Literal struct {
	Kind LiteralKind
	Text string
	N    int
}

type SelectItem interface{ selectItem() }
type ValueExpr interface{ valueExpr() }
type Expr interface{ expr() }

type FieldRef struct {
	Path []string
	Pos  Pos
}

type SubQuery struct {
	Query *Query
	Pos   Pos
}

type Aggregate struct {
	Func  string
	Arg   *FieldRef
	Alias string
	Pos   Pos
}

type Call struct {
	Func  string
	Args  []ValueExpr
	Alias string
	Pos   Pos
}

type FieldsAll struct {
	Kind string
	Pos  Pos
}

type TypeOf struct {
	Rel   Name
	Whens []TypeOfWhen
	Else  []ValueExpr
	Pos   Pos
}

type TypeOfWhen struct {
	Type   Name        `json:"type"`
	Fields []ValueExpr `json:"fields"`
}

type And struct {
	Items []Expr
	Pos   Pos
}

type Or struct {
	Items []Expr
	Pos   Pos
}

type Not struct {
	Expr Expr
	Pos  Pos
}

type Compare struct {
	Left  ValueExpr
	Op    string
	Right Literal
	Pos   Pos
}

type In struct {
	Left   ValueExpr
	Not    bool
	Values []Literal
	Sub    *Query
	Pos    Pos
}

type Includes struct {
	Left    *FieldRef
	Exclude bool
	Values  []Literal
	Pos     Pos
}

func (*FieldRef) selectItem()  {}
func (*SubQuery) selectItem()  {}
func (*Aggregate) selectItem() {}
func (*Call) selectItem()      {}
func (*FieldsAll) selectItem() {}
func (*TypeOf) selectItem()    {}

func (*FieldRef) valueExpr()  {}
func (*Aggregate) valueExpr() {}
func (*Call) valueExpr()      {}

func (*And) expr()      {}
func (*Or) expr()       {}
func (*Not) expr()      {}
func (*Compare) expr()  {}
func (*In) expr()       {}
func (*Includes) expr() {}

func (n NullPlacement) String() string {
	switch n {
	case NullFirst:
		return "first"
	case NullLast:
		return "last"
	}
	return "default"
}

func (k LiteralKind) String() string {
	switch k {
	case LitNumber:
		return "number"
	case LitBool:
		return "bool"
	case LitNull:
		return "null"
	case LitDate:
		return "date"
	case LitDateTime:
		return "dateTime"
	case LitDateLiteral:
		return "dateLiteral"
	}
	return "string"
}

// Golden JSON uses a "kind" discriminator and omits source positions.

func (f *FieldRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string   `json:"kind"`
		Path []string `json:"path"`
	}{"field", f.Path})
}

func (s *SubQuery) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string `json:"kind"`
		Query *Query `json:"query"`
	}{"subquery", s.Query})
}

func (a *Aggregate) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string    `json:"kind"`
		Func  string    `json:"func"`
		Arg   *FieldRef `json:"arg"`
		Alias string    `json:"alias,omitempty"`
	}{"agg", a.Func, a.Arg, a.Alias})
}

func (c *Call) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string      `json:"kind"`
		Func  string      `json:"func"`
		Args  []ValueExpr `json:"args"`
		Alias string      `json:"alias,omitempty"`
	}{"call", c.Func, c.Args, c.Alias})
}

func (f *FieldsAll) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Arg  string `json:"arg"`
	}{"fields", f.Kind})
}

func (t *TypeOf) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string       `json:"kind"`
		Rel   Name         `json:"rel"`
		Whens []TypeOfWhen `json:"whens"`
		Else  []ValueExpr  `json:"else"`
	}{"typeof", t.Rel, t.Whens, t.Else})
}

func (a *And) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string `json:"kind"`
		Items []Expr `json:"items"`
	}{"and", a.Items})
}

func (o *Or) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string `json:"kind"`
		Items []Expr `json:"items"`
	}{"or", o.Items})
}

func (n *Not) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Expr Expr   `json:"expr"`
	}{"not", n.Expr})
}

func (c *Compare) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind  string    `json:"kind"`
		Left  ValueExpr `json:"left"`
		Op    string    `json:"op"`
		Right Literal   `json:"right"`
	}{"compare", c.Left, c.Op, c.Right})
}

func (i *In) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind   string    `json:"kind"`
		Left   ValueExpr `json:"left"`
		Not    bool      `json:"not"`
		Values []Literal `json:"values"`
		Sub    *Query    `json:"sub"`
	}{"in", i.Left, i.Not, i.Values, i.Sub})
}

func (i *Includes) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind    string    `json:"kind"`
		Left    *FieldRef `json:"left"`
		Exclude bool      `json:"exclude"`
		Values  []Literal `json:"values"`
	}{"includes", i.Left, i.Exclude, i.Values})
}

func (l Literal) MarshalJSON() ([]byte, error) {
	if l.Kind == LitDateLiteral {
		return json.Marshal(struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
			N    int    `json:"n"`
		}{l.Kind.String(), l.Text, l.N})
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
		Text string `json:"text"`
	}{l.Kind.String(), l.Text})
}

func (o OrderItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Value ValueExpr     `json:"value"`
		Desc  bool          `json:"desc"`
		Nulls NullPlacement `json:"nulls"`
	}{o.Value, o.Desc, o.Nulls})
}

func (n NullPlacement) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

func (q *Query) MarshalJSON() ([]byte, error) {
	type plain Query
	return json.Marshal((*plain)(q))
}
