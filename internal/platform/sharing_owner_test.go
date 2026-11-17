package platform

import (
	"context"
	"errors"
	"testing"

	"salescompel/internal/metadata"
	soqlpkg "salescompel/internal/soql"
)

// sharingUser creates a CLI user on profile and returns their ID.
func sharingUser(t *testing.T, f writeFixture, name, profile string) string {
	t.Helper()
	id, e := f.s.CreateUserWithProfile(context.Background(), f.tenant, name+"."+f.tenant+"@example.com", "long-password", profile)
	if e != nil {
		t.Fatal(e)
	}
	return id
}

// as runs fn in user's request transaction.
func as(t *testing.T, f writeFixture, user string, fn func(*Store, Principal) error) error {
	t.Helper()
	return f.s.within(context.Background(), Principal{Tenant: f.tenant, User: user}, fn)
}

func createAs(t *testing.T, f writeFixture, user, object string, data map[string]any) (string, error) {
	t.Helper()
	var id string
	err := as(t, f, user, func(s *Store, p Principal) error {
		var e error
		id, e = s.recordTx(p).Create(context.Background(), object, data)
		return e
	})
	return id, err
}

func ownerOf(t *testing.T, f writeFixture, object, id string) string {
	t.Helper()
	var owner string
	if e := as(t, f, f.user, func(s *Store, p Principal) error {
		res, e := s.recordTx(p).Query(context.Background(), "SELECT OwnerId, Owner.Name FROM "+object+" WHERE Id = '"+id+"'", soqlpkg.Options{})
		if e != nil {
			return e
		}
		if len(res.Records) != 1 {
			t.Fatalf("%s %s not found", object, id)
		}
		r := res.Records[0]
		owner, _ = r["OwnerId"].(string)
		if o, _ := r["Owner"].(map[string]any); o == nil || o["Name"] == nil {
			t.Fatalf("Owner relationship missing: %v", r)
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	return owner
}

func wantCode(t *testing.T, what string, err error, code string) {
	t.Helper()
	var api *APIError
	if !errors.As(err, &api) || api.Code != code {
		t.Fatalf("%s: got %v, want %s", what, err, code)
	}
}

// shr-01: OwnerId defaults to the creating user; Owner traverses to User.
func TestOwnerDefaultOnCreate(t *testing.T) {
	f := newWriteFixture(t)
	std := sharingUser(t, f, "owner-std", "Standard User")
	id, e := createAs(t, f, std, "Account", map[string]any{"Name": "Owned"})
	if e != nil {
		t.Fatal(e)
	}
	if got := ownerOf(t, f, "Account", id); got != std {
		t.Fatalf("owner %s, want %s", got, std)
	}
	if _, e := metadata.CreateObject(context.Background(), f.s.pool, f.tenant, metadata.ObjectInput{Name: "Owned__c", Label: "Owned", PluralLabel: "Owned"}); e != nil {
		t.Fatal(e)
	}
	cid, e := createAs(t, f, f.user, "Owned__c", map[string]any{"Name": "c"})
	if e != nil {
		t.Fatal(e)
	}
	if got := ownerOf(t, f, "Owned__c", cid); got != f.user {
		t.Fatalf("custom owner %s", got)
	}
}

// shr-01, D34-09: a named owner must be an active user who can read the
// object; a Group ID is refused.
func TestOwnerGivenMustBeActiveReader(t *testing.T) {
	f := newWriteFixture(t)
	std := sharingUser(t, f, "given-std", "Standard User")
	min := sharingUser(t, f, "given-min", "Minimum Access")
	id, e := createAs(t, f, f.user, "Account", map[string]any{"Name": "Given", "OwnerId": std})
	if e != nil || ownerOf(t, f, "Account", id) != std {
		t.Fatalf("given owner: %v", e)
	}
	_, e = createAs(t, f, f.user, "Account", map[string]any{"Name": "No read", "OwnerId": min})
	wantCode(t, "owner without Read", e, "INVALID_CROSS_REFERENCE_KEY")
	if _, e := f.s.pool.Exec(context.Background(), "UPDATE sc_setup.users SET is_active=false WHERE id=$1", std); e != nil {
		t.Fatal(e)
	}
	_, e = createAs(t, f, f.user, "Account", map[string]any{"Name": "Inactive", "OwnerId": std})
	wantCode(t, "inactive owner", e, "INVALID_CROSS_REFERENCE_KEY")
	_, e = createAs(t, f, f.user, "Account", map[string]any{"Name": "Queue", "OwnerId": "00G000000000001AAA"})
	wantCode(t, "group owner", e, "MALFORMED_ID")
}

// D34-31: a provisioned static-token principal's records go to the tenant's
// default owner, the System Administrator.
func TestOwnerStaticPrincipalDefaultOwner(t *testing.T) {
	f := newWriteFixture(t)
	sharingUser(t, f, "static-std", "Standard User")
	var id string
	if e := f.s.within(context.Background(), Principal{Tenant: f.tenant, User: "static-token", Role: "restricted"}, func(s *Store, p Principal) error {
		var e error
		id, e = s.recordTx(p).Create(context.Background(), "Account", map[string]any{"Name": "Static"})
		return e
	}); e != nil {
		t.Fatal(e)
	}
	if got := ownerOf(t, f, "Account", id); got != f.user {
		t.Fatalf("static owner %s, want %s", got, f.user)
	}
}
