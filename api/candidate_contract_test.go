package apicontract

import (
	"os"
	"testing"
)

func TestCandidateOpenAPIMatchesSource(t *testing.T) {
	source, err := os.ReadFile("candidate-openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(source) != CandidateOpenAPI() {
		t.Fatal("binary contract differs from canonical source")
	}
}
