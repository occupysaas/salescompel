package apicontract

import (
	"os"
	"strings"
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

func TestCandidateOAuthContractInventory(t *testing.T) {
	contract := CandidateOpenAPI()
	for _, operation := range []string{"candidateOAuthAuthorizeForm", "candidateOAuthAuthorizeSubmit", "candidateOAuthToken", "candidateOAuthRevoke", "candidateOAuthIntrospect", "candidateOAuthDiscovery", "candidateOAuthIdentity"} {
		if !strings.Contains(contract, "operationId: "+operation) {
			t.Fatalf("missing OAuth operation %s", operation)
		}
	}
	for _, marker := range []string{"openapi: 3.2.1", "authorizationCode:", "clientCredentials:", "OrganizationProjection:", "OAuthError:"} {
		if !strings.Contains(contract, marker) {
			t.Fatalf("missing contract marker %s", marker)
		}
	}
}
