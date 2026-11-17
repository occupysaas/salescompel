package platform

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
)

var errRollback = errors.New("rollback")

// tryAs runs fn as user and always rolls back, returning fn's error.
func tryAs(t *testing.T, f writeFixture, user string, fn func(*Store, Principal) error) error {
	t.Helper()
	var got error
	e := as(t, f, user, func(s *Store, p Principal) error {
		got = fn(s, p)
		return errRollback
	})
	if e != nil && !errors.Is(e, errRollback) {
		return e
	}
	return got
}

// matrixWorld is the independent model the matrix checks against: who owns
// what, the hierarchy, group membership, shares and rules, evaluated with
// plain Go maps rather than the SQL predicate.
type matrixWorld struct {
	admin       string
	viewAll     string
	subordinate map[string][]string // user -> users in roles below
	groups      map[string][]string // user -> groups
	owner       map[string]string   // record -> owner
	shares      map[string]map[string]string
	rules       []struct{ source, target, level string }
}

func (w matrixWorld) principals(user string) map[string]bool {
	out := map[string]bool{user: true}
	for _, g := range w.groups[user] {
		out[g] = true
	}
	for _, s := range w.subordinate[user] {
		out[s] = true
		for _, g := range w.groups[s] {
			out[g] = true
		}
	}
	return out
}

func (w matrixWorld) allows(user, record, owd, level string) bool {
	if user == w.admin || (user == w.viewAll && level == "Read") {
		return true
	}
	pr := w.principals(user)
	if pr[w.owner[record]] {
		return true
	}
	if level == "Delete" {
		return false
	}
	if owd == "Edit" || owd == "Read" && level == "Read" {
		return true
	}
	ok := func(l string) bool { return l == "Edit" || level == "Read" && l == "Read" }
	for grantee, l := range w.shares[record] {
		if pr[grantee] && ok(l) {
			return true
		}
	}
	for _, r := range w.rules {
		inSource := false
		for _, g := range w.groups[w.owner[record]] {
			inSource = inSource || g == r.source
		}
		if inSource && pr[r.target] && ok(r.level) {
			return true
		}
	}
	return false
}

// TestSharingMatrix: every user × account × level × org-wide default agrees
// with the model on SOQL, REST read, update and delete. No unexpected grant
// and no unexpected denial.
func TestSharingMatrix(t *testing.T) {
	f := newWriteFixture(t)
	ctx := context.Background()
	boss, mid, rep, peer := hierarchy(t, f)
	outsider := sharingUser(t, f, "mx-outsider", "Standard User")
	viewer := sharingUser(t, f, "mx-viewer", "Standard User")
	set := f.created("PermissionSet", `{"Name":"View_accounts","Label":"View accounts"}`)
	f.created("ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"Account","PermissionsRead":true,"PermissionsViewAllRecords":true}`)
	f.created("PermissionSetAssignment", `{"AssigneeId":"`+viewer+`","PermissionSetId":"`+set+`"}`)
	team := f.created("Group", `{"Name":"Team","DeveloperName":"Team"}`)
	f.created("GroupMember", `{"GroupId":"`+team+`","UserOrGroupId":"`+outsider+`"}`)
	peers := f.created("Group", `{"Name":"Peers","DeveloperName":"Peers"}`)
	f.created("GroupMember", `{"GroupId":"`+peers+`","UserOrGroupId":"`+peer+`"}`)

	w := matrixWorld{admin: f.user, viewAll: viewer, owner: map[string]string{}, shares: map[string]map[string]string{},
		subordinate: map[string][]string{boss: {mid, rep, peer}, mid: {rep, peer}},
		groups:      map[string][]string{outsider: {team}, peer: {peers}}}
	users := []string{f.user, boss, mid, rep, peer, outsider, viewer}
	var records []string
	for _, owner := range []string{rep, peer, mid, outsider} {
		id := mustCreate(t, f, owner, "Account", map[string]any{"Name": "Matrix " + owner[len(owner)-6:]})
		records = append(records, id)
		w.owner[id] = owner
	}
	// rep's account shared with the Team group (Read); mid's with peer (Edit).
	manualShare(t, f, "Account", records[0], team, "Read")
	manualShare(t, f, "Account", records[2], peer, "Edit")
	w.shares[records[0]] = map[string]string{team: "Read"}
	w.shares[records[2]] = map[string]string{peer: "Edit"}
	// Records owned by Peers members are shared with Team at Edit.
	if _, e := f.s.AddSharingRule(ctx, f.tenant, SharingRule{Object: "Account", Name: "Peers to team", Source: peers, Target: team, Access: "Edit"}); e != nil {
		t.Fatal(e)
	}
	w.rules = append(w.rules, struct{ source, target, level string }{peers, team, "Edit"})

	cells := 0
	var wrong []string
	for _, owd := range []string{"None", "Read", "Edit"} {
		setOWD(t, f, "Account", owd)
		for _, u := range users {
			for _, rec := range records {
				read := queryAs(t, f, u, "SELECT Id FROM Account WHERE Id = '"+rec+"'").TotalSize == 1
				restErr := tryAs(t, f, u, func(s *Store, p Principal) error {
					_, e := s.recordTx(p).Read(ctx, "Account", rec)
					return e
				})
				editErr := tryAs(t, f, u, func(s *Store, p Principal) error {
					return s.recordTx(p).Update(ctx, "Account", rec, map[string]any{"Description": "matrix"})
				})
				delErr := tryAs(t, f, u, func(s *Store, p Principal) error { return s.recordTx(p).Delete(ctx, "Account", rec) })
				got := map[string]bool{"Read": read, "REST read": restErr == nil, "Edit": editErr == nil, "Delete": delErr == nil}
				want := map[string]bool{"Read": w.allows(u, rec, owd, "Read"), "Edit": w.allows(u, rec, owd, "Edit"), "Delete": w.allows(u, rec, owd, "Delete")}
				want["REST read"] = want["Read"]
				for _, k := range []string{"Read", "REST read", "Edit", "Delete"} {
					cells++
					if got[k] != want[k] {
						wrong = append(wrong, fmt.Sprintf("owd=%s user=%s record=%s %s: got %v want %v", owd, u, rec, k, got[k], want[k]))
					}
				}
				// A denied read is NOT_FOUND; a readable but denied write is
				// INSUFFICIENT_ACCESS_OR_READONLY.
				if !want["Edit"] {
					code := "NOT_FOUND"
					if want["Read"] {
						code = "INSUFFICIENT_ACCESS_OR_READONLY"
					}
					var api *APIError
					if !errors.As(editErr, &api) || api.Code != code {
						wrong = append(wrong, fmt.Sprintf("owd=%s user=%s record=%s edit error %v, want %s", owd, u, rec, editErr, code))
					}
				}
			}
		}
	}
	sort.Strings(wrong)
	for _, line := range wrong {
		t.Error(line)
	}
	if cells != 3*len(users)*len(records)*4 {
		t.Fatalf("checked %d cells", cells)
	}
}
