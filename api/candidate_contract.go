// Package apicontract publishes the independently authored candidate API contract.
package apicontract

import _ "embed"

const CandidateOpenAPIPath = "/services/data/v65.0/openapi.yaml"
const CandidateOpenAPIContentType = "application/yaml; charset=utf-8"

//go:embed candidate-openapi.yaml
var candidateOpenAPI string

// CandidateOpenAPI returns immutable bytes embedded at build time.
func CandidateOpenAPI() string { return candidateOpenAPI }
