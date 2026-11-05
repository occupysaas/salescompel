package main

import (
	"bytes"
	"testing"
)

func TestMigrateCommandRequiresOwnerURL(t *testing.T) {
	t.Setenv("MIGRATION_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://unused.invalid/no")
	var out bytes.Buffer
	if e := dispatch([]string{"migrate", "status"}, &out); e == nil || e.Error() != "MIGRATION_DATABASE_URL required" {
		t.Fatalf("unexpected result %v", e)
	}
}
