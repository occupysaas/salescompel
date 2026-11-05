package metadata

import (
	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
	"strings"
)

type Field struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Nillable    bool     `json:"nillable"`
	Createable  bool     `json:"createable"`
	Updateable  bool     `json:"updateable"`
	ExternalID  bool     `json:"externalId"`
	Length      int      `json:"length"`
	ReferenceTo []string `json:"referenceTo"`
	Column      string   `json:"-"`
	ID          string   `json:"-"`
}
type Object struct {
	ID, Tenant, Schema, Table, Prefix string  `json:"-"`
	PluralLabel                       string  `json:"labelPlural"`
	Standard                          bool    `json:"-"`
	Name                              string  `json:"name"`
	Label                             string  `json:"label"`
	Fields                            []Field `json:"fields"`
	Createable                        bool    `json:"createable"`
	Updateable                        bool    `json:"updateable"`
	Deletable                         bool    `json:"deletable"`
	Queryable                         bool    `json:"queryable"`
}

type Snapshot struct {
	Tenant   string
	Revision int64
	Objects  []Object
}

func (o Object) TableSQL() string { return pgx.Identifier{o.Schema, o.Table}.Sanitize() }
func (s Snapshot) Object(name string) (Object, error) {
	for _, o := range s.Objects {
		if strings.EqualFold(o.Name, name) {
			return o, nil
		}
	}
	return Object{}, contract.Invalid("INVALID_TYPE", "Unsupported object")
}
func (s Snapshot) Field(object, name string) (Field, error) {
	o, e := s.Object(object)
	if e != nil {
		return Field{}, e
	}
	for _, f := range o.Fields {
		if strings.EqualFold(f.Name, name) {
			return f, nil
		}
	}
	return Field{}, contract.FieldProblem("INVALID_FIELD", "Field is unavailable", name)
}
