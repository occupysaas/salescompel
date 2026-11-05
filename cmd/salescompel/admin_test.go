package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAdminCommands(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("SALESCOMPEL_TENANT", fmt.Sprintf("admin-%d", time.Now().UnixNano()))
	t.Setenv("SALESCOMPEL_USERNAME", os.Getenv("SALESCOMPEL_TENANT")+"@example.com")
	t.Setenv("SALESCOMPEL_PASSWORD", "secret-password")
	t.Setenv("SALESCOMPEL_ROLE", "admin")
	call := func(args ...string) (string, error) {
		var out bytes.Buffer
		e := dispatch(args, &out)
		if e != nil && strings.Contains(e.Error(), "secret-password") {
			t.Fatal("password leak")
		}
		return out.String(), e
	}
	out, e := call("user", "create")
	if e != nil || !strings.HasPrefix(out, "005") {
		t.Fatal(out, e)
	}
	out, e = call("oauth-app", "create", "--tenant", os.Getenv("SALESCOMPEL_TENANT"), "--name", "example")
	var app map[string]string
	if json.Unmarshal([]byte(out), &app) != nil || e != nil || len(app["client_secret"]) != 64 || app["client_id"] == "" {
		t.Fatal(out, e)
	}
	out, e = call("oauth-app", "rotate-secret", "--client-id", app["client_id"])
	if e != nil || len(strings.TrimSpace(out)) != 64 || strings.TrimSpace(out) == app["client_secret"] {
		t.Fatal(out, e)
	}
	out, e = call("oauth-app", "create", "--tenant", os.Getenv("SALESCOMPEL_TENANT"), "--name", "public", "--public")
	if e != nil || strings.Contains(out, "client_secret") {
		t.Fatal(out, e)
	}
	t.Setenv("SALESCOMPEL_PASSWORD", "")
	if _, e = call("user", "create"); e == nil {
		t.Fatal("missing password accepted")
	}
	if _, e = call("user", "create", "--password", "secret-password"); e == nil {
		t.Fatal("password flag accepted")
	}
}
