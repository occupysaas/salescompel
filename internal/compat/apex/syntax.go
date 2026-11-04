package apex

type token struct {
	text, kind   string
	line, column int
}

func (t token) err(code string) error { return &Diagnostic{Code: code, Line: t.line, Column: t.column} }

type parameter struct{ typ, name string }
type program struct {
	class  string
	params []parameter
	body   *statement
}
type expression struct {
	tok         token
	op          string
	literal     Value
	name        string
	left, right *expression
	height      int
}
type statement struct {
	tok               token
	op, typ, name     string
	expr, target      *expression
	body, alternative *statement
	statements        []*statement
}
