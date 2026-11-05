package platform

import (
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/recordid"
	"salescompel/internal/records"
)

type Principal = policy.Principal
type APIError = contract.APIError
type Field = metadata.Field
type Object = metadata.Object

var problem = contract.Problem
var invalid = contract.Invalid
var fieldProblem = contract.FieldProblem
var dbError = records.DBError
var idChecksum = recordid.Checksum
