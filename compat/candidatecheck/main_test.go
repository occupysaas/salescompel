package main

import (
	"context"
	"testing"
)

func TestVersionMajor(t *testing.T) {
	if major(170006) != 17 || major(160000) != 16 {
		t.Fatal("version")
	}
}
func TestInvalidDatabaseIsSanitized(t *testing.T) {
	_, err := check(context.Background(), "invalid secret-canary")
	if err == nil || err.Error() != "DATABASE_UNAVAILABLE" {
		t.Fatal("unsafe failure")
	}
}
