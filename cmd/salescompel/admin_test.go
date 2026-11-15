package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"salescompel/internal/migration"
	"strings"
	"testing"
	"time"
)

func TestAdminCommands(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	t.Setenv("MIGRATION_DATABASE_URL", dsn)
	pool, e := pgxpool.New(context.Background(), dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if e = migration.Up(context.Background(), pool); e != nil {
		t.Fatal(e)
	}
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

// adminFixture migrates the test database and sets a fresh tenant and
// username; it returns a pool for inspecting the result.
func adminFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	t.Setenv("MIGRATION_DATABASE_URL", dsn)
	pool, e := pgxpool.New(context.Background(), dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	if e = migration.Up(context.Background(), pool); e != nil {
		t.Fatal(e)
	}
	tenant := fmt.Sprintf("org-%d", time.Now().UnixNano())
	t.Setenv("SALESCOMPEL_TENANT", tenant)
	t.Setenv("SALESCOMPEL_USERNAME", "first@"+tenant+".example.com")
	t.Setenv("SALESCOMPEL_PASSWORD", "secret-password")
	t.Setenv("SALESCOMPEL_ROLE", "")
	t.Setenv("SALESCOMPEL_PROFILE", "System Administrator")
	return pool
}

func userSetting(t *testing.T, pool *pgxpool.Pool, id string) (zone, locale, profile string) {
	t.Helper()
	e := pool.QueryRow(context.Background(), `SELECT u.time_zone_sid_key,u.locale_sid_key,p.name FROM sc_setup.users u
		JOIN sc_setup.profiles p ON p.tenant_id=u.tenant_id AND p.id=u.profile_id WHERE u.id=$1`, id).Scan(&zone, &locale, &profile)
	if e != nil {
		t.Fatal(e)
	}
	return
}

// org set changes the org defaults a new CLI user starts from; existing
// users keep their settings.
func TestOrgSetChangesDefaultsNotUsers(t *testing.T) {
	pool := adminFixture(t)
	var out bytes.Buffer
	if e := dispatch([]string{"user", "create"}, &out); e != nil {
		t.Fatal(e)
	}
	first := strings.TrimSpace(out.String())
	t.Setenv("SALESCOMPEL_TIME_ZONE", "Europe/Paris")
	t.Setenv("SALESCOMPEL_LOCALE", "fr_FR")
	t.Setenv("SALESCOMPEL_LANGUAGE", "fr")
	if e := dispatch([]string{"org", "set"}, &out); e != nil {
		t.Fatal(e)
	}
	var zone, locale, language string
	if e := pool.QueryRow(context.Background(), "SELECT time_zone_sid_key,default_locale_sid_key,language_locale_key FROM sc_catalog.settings WHERE tenant_id=$1",
		os.Getenv("SALESCOMPEL_TENANT")).Scan(&zone, &locale, &language); e != nil || zone != "Europe/Paris" || locale != "fr_FR" || language != "fr" {
		t.Fatalf("settings %s %s %s %v", zone, locale, language, e)
	}
	t.Setenv("SALESCOMPEL_USERNAME", "second@"+os.Getenv("SALESCOMPEL_TENANT")+".example.com")
	out.Reset()
	if e := dispatch([]string{"user", "create"}, &out); e != nil {
		t.Fatal(e)
	}
	if z, l, _ := userSetting(t, pool, strings.TrimSpace(out.String())); z != "Europe/Paris" || l != "fr_FR" {
		t.Fatalf("new user %s %s", z, l)
	}
	if z, l, _ := userSetting(t, pool, first); z != "America/Los_Angeles" || l != "en_US" {
		t.Fatalf("existing user %s %s", z, l)
	}
	// Only the values given change; a bad value changes nothing.
	t.Setenv("SALESCOMPEL_TIME_ZONE", "")
	t.Setenv("SALESCOMPEL_LOCALE", "")
	t.Setenv("SALESCOMPEL_LANGUAGE", "de")
	if e := dispatch([]string{"org", "set"}, &out); e != nil {
		t.Fatal(e)
	}
	t.Setenv("SALESCOMPEL_TIME_ZONE", "Mars/Olympus")
	if e := dispatch([]string{"org", "set"}, &out); e == nil {
		t.Fatal("unknown time zone accepted")
	}
	pool.QueryRow(context.Background(), "SELECT time_zone_sid_key,language_locale_key FROM sc_catalog.settings WHERE tenant_id=$1", os.Getenv("SALESCOMPEL_TENANT")).Scan(&zone, &language)
	if zone != "Europe/Paris" || language != "de" {
		t.Fatalf("after partial set: %s %s", zone, language)
	}
	if e := dispatch([]string{"org", "set", "--time-zone", "UTC"}, &out); e == nil {
		t.Fatal("org set accepted arguments")
	}
}

// SALESCOMPEL_ROLE still works when SALESCOMPEL_PROFILE is unset, with a
// deprecation notice on stderr.
func TestUserCreateRoleAlias(t *testing.T) {
	pool := adminFixture(t)
	t.Setenv("SALESCOMPEL_PROFILE", "")
	t.Setenv("SALESCOMPEL_ROLE", "restricted")
	var out, notice bytes.Buffer
	stderr = &notice
	t.Cleanup(func() { stderr = os.Stderr })
	if e := dispatch([]string{"user", "create"}, &out); e != nil {
		t.Fatal(e)
	}
	if _, _, p := userSetting(t, pool, strings.TrimSpace(out.String())); p != "Standard User" {
		t.Fatalf("profile %s", p)
	}
	if !strings.Contains(notice.String(), "SALESCOMPEL_ROLE is deprecated") {
		t.Fatalf("notice %q", notice.String())
	}
	// SALESCOMPEL_PROFILE wins and takes any profile, with no notice.
	notice.Reset()
	t.Setenv("SALESCOMPEL_PROFILE", "Read Only")
	t.Setenv("SALESCOMPEL_USERNAME", "reader@"+os.Getenv("SALESCOMPEL_TENANT")+".example.com")
	out.Reset()
	if e := dispatch([]string{"user", "create"}, &out); e != nil {
		t.Fatal(e)
	}
	if _, _, p := userSetting(t, pool, strings.TrimSpace(out.String())); p != "Read Only" || notice.Len() != 0 {
		t.Fatalf("profile %s notice %q", p, notice.String())
	}
	t.Setenv("SALESCOMPEL_PROFILE", "No Such Profile")
	t.Setenv("SALESCOMPEL_USERNAME", "nobody@"+os.Getenv("SALESCOMPEL_TENANT")+".example.com")
	if e := dispatch([]string{"user", "create"}, &out); e == nil {
		t.Fatal("unknown profile accepted")
	}
}
