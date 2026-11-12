// Package soql lexes and parses the documented SOQL subset into an AST.
package soql

import (
	"salescompel/internal/contract"
	"strings"
)

// Pos is a byte offset in the query text.
type Pos struct{ Offset int }

type TokenKind int

const (
	Ident TokenKind = iota
	String
	Number
	Date
	DateTime
	DateLiteral
	Op
	LParen
	RParen
	Comma
	Keyword
	EOF
)

type Token struct {
	Kind TokenKind
	Text string
	Pos  Pos
}

// Name is an object or relationship name as written in the query.
type Name string

// malformed builds the closed-vocabulary syntax error. Messages name
// constructs only: they never echo query values.
// spec: soql-06
func malformed(msg string) error { return contract.Invalid("MALFORMED_QUERY", msg) }

var keywordSet = map[string]bool{
	"select": true, "from": true, "where": true, "group": true, "by": true,
	"having": true, "order": true, "asc": true, "desc": true, "nulls": true,
	"first": true, "last": true, "limit": true, "offset": true, "with": true,
	"and": true, "or": true, "not": true, "in": true, "like": true,
	"includes": true, "excludes": true, "typeof": true, "when": true,
	"then": true, "else": true, "end": true, "as": true, "null": true,
}

// Clause keywords must not stand in for names, so a duplicated or misplaced
// clause is an error rather than an identifier. "order" is deliberately
// absent: it is the standard object name Order.
// spec: soql-06
var clauseKeyword = map[string]bool{
	"select": true, "from": true, "where": true, "group": true, "by": true,
	"having": true, "limit": true, "offset": true, "with": true,
}

// spec: soql-04
var dateLiteral = map[string]bool{
	"yesterday": true, "today": true, "tomorrow": true,
	"last_week": true, "this_week": true, "next_week": true,
	"last_month": true, "this_month": true, "next_month": true,
	"last_quarter": true, "this_quarter": true, "next_quarter": true,
	"last_year": true, "this_year": true, "next_year": true,
	"last_90_days": true, "next_90_days": true,
	"last_fiscal_quarter": true, "this_fiscal_quarter": true, "next_fiscal_quarter": true,
	"last_fiscal_year": true, "this_fiscal_year": true, "next_fiscal_year": true,
}

// spec: soql-04
var dateLiteralN = map[string]bool{
	"last_n_days": true, "next_n_days": true, "n_days_ago": true,
	"last_n_weeks": true, "next_n_weeks": true, "n_weeks_ago": true,
	"last_n_months": true, "next_n_months": true, "n_months_ago": true,
	"last_n_quarters": true, "next_n_quarters": true, "n_quarters_ago": true,
	"last_n_years": true, "next_n_years": true, "n_years_ago": true,
	"last_n_fiscal_quarters": true, "next_n_fiscal_quarters": true, "n_fiscal_quarters_ago": true,
	"last_n_fiscal_years": true, "next_n_fiscal_years": true, "n_fiscal_years_ago": true,
}

// spec: soql-02
var dateFunc = map[string]bool{
	"calendar_year": true, "calendar_quarter": true, "calendar_month": true,
	"day_in_month": true, "day_in_week": true, "day_in_year": true,
	"day_only": true, "hour_in_day": true, "week_in_month": true, "week_in_year": true,
	"fiscal_month": true, "fiscal_quarter": true, "fiscal_year": true,
}

// spec: soql-02
var aggFunc = map[string]bool{
	"count": true, "count_distinct": true, "sum": true, "avg": true,
	"min": true, "max": true,
}

func isDateLiteral(word string) bool {
	lower := strings.ToLower(word)
	return dateLiteral[lower] || dateLiteralN[lower]
}

func isDateLiteralN(word string) bool { return dateLiteralN[strings.ToLower(word)] }
