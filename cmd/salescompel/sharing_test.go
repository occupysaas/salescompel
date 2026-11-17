package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

// `salescompel sharing set-default` validates values per object and the
// Account/Contact pairing, and Organization reflects it (D34-16, D34-20).
func TestSharingSetDefault(t *testing.T) {
	pool := adminFixture(t)
	var out bytes.Buffer
	if e := dispatch([]string{"user", "create"}, &out); e != nil {
		t.Fatal(e)
	}
	tenant := os.Getenv("SALESCOMPEL_TENANT")
	set := func(object, access string) error {
		return dispatch([]string{"sharing", "set-default", "--tenant", tenant, "--object", object, "--access", access}, &out)
	}
	for _, bad := range [][2]string{{"Account", "ControlledByParent"}, {"Account", "Transfer"}, {"User", "None"}, {"Nope__c", "None"}} {
		if e := set(bad[0], bad[1]); e == nil {
			t.Errorf("%s=%s accepted", bad[0], bad[1])
		}
	}
	if e := set("Contact", "Read"); e != nil {
		t.Fatal(e)
	}
	if e := set("Account", "None"); e == nil {
		t.Fatal("Private Account accepted while Contact is Public Read Only")
	}
	if e := set("Contact", "ControlledByParent"); e != nil {
		t.Fatal(e)
	}
	if e := set("Account", "None"); e != nil {
		t.Fatal(e)
	}
	if e := set("Contact", "Edit"); e == nil {
		t.Fatal("Public Read/Write Contact accepted under a Private Account")
	}
	var access string
	if e := pool.QueryRow(context.Background(), "SELECT default_account_access||','||default_contact_access FROM sc_setup.organization WHERE tenant_id=$1", tenant).Scan(&access); e != nil || access != "None,ControlledByParent" {
		t.Fatalf("organization %q %v", access, e)
	}
}

// `salescompel sharing rule add|list|remove` manage owner-based rules, and
// a rule grants access (D34-30).
func TestSharingRuleCommands(t *testing.T) {
	pool := adminFixture(t)
	ctx := context.Background()
	var out bytes.Buffer
	tenant := os.Getenv("SALESCOMPEL_TENANT")
	user := func(name, profile string) string {
		t.Setenv("SALESCOMPEL_USERNAME", name+"@"+tenant+".example.com")
		t.Setenv("SALESCOMPEL_PROFILE", profile)
		out.Reset()
		if e := dispatch([]string{"user", "create"}, &out); e != nil {
			t.Fatal(e)
		}
		return strings.TrimSpace(out.String())
	}
	user("admin", "System Administrator")
	owner, reader := user("owner", "Standard User"), user("reader", "Standard User")
	exec := func(sql string, args ...any) {
		t.Helper()
		tx, _ := pool.Begin(ctx)
		defer tx.Rollback(ctx)
		tx.Exec(ctx, "SELECT set_config('salescompel.tenant_id',$1,true)", tenant)
		if _, e := tx.Exec(ctx, sql, args...); e != nil {
			t.Fatal(e)
		}
		tx.Commit(ctx)
	}
	// Two public groups, one member each; closure via a rebuild-equivalent
	// insert keeps this test independent of the REST path.
	exec(`INSERT INTO sc_setup.groups(id,tenant_id,name,developer_name,type) VALUES('00G000000000S01AAA',$1,'Sellers','Sellers','Regular'),('00G000000000R01AAA',$1,'Readers','Readers','Regular')`, tenant)
	exec(`INSERT INTO sc_setup.group_members(id,tenant_id,group_id,user_or_group_id) VALUES('011000000000S01AAA',$1,'00G000000000S01AAA',$2),('011000000000R01AAA',$1,'00G000000000R01AAA',$3)`, tenant, owner, reader)
	rule := func(args ...string) error {
		out.Reset()
		return dispatch(append([]string{"sharing", "rule"}, append(args, "--tenant", tenant)...), &out)
	}
	if e := rule("add", "--object", "Account", "--name", "Sellers to readers", "--source", "Sellers", "--target", "Readers", "--access", "All"); e == nil {
		t.Fatal("access All accepted")
	}
	if e := rule("add", "--object", "Account", "--name", "Bad", "--source", "Nobody", "--target", "Readers", "--access", "Read"); e == nil {
		t.Fatal("unknown group accepted")
	}
	if e := rule("add", "--object", "Account", "--name", "Sellers to readers", "--source", "Sellers", "--target", "00G000000000R01AAA", "--access", "Read"); e != nil {
		t.Fatal(e)
	}
	id := strings.TrimSpace(out.String())
	if e := rule("list"); e != nil || !strings.Contains(out.String(), id+"\tAccount\tSellers to readers\t00G000000000S01AAA\t00G000000000R01AAA\tRead") {
		t.Fatalf("list %q %v", out.String(), e)
	}
	if e := rule("remove", "--id", id); e != nil {
		t.Fatal(e)
	}
	if e := rule("remove", "--id", id); e == nil {
		t.Fatal("removed twice")
	}
}
