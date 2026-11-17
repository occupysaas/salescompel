package platform

import (
	"context"
	"testing"

	"salescompel/internal/metadata"
)

// D34-30: an owner-based rule shares records owned by one group's members
// with another group's members, at the rule's level only.
func TestSharingRuleGrantsAccess(t *testing.T) {
	f := newWriteFixture(t)
	owner, reader := sharingUser(t, f, "rule-owner", "Standard User"), sharingUser(t, f, "rule-reader", "Standard User")
	sellers := f.created("Group", `{"Name":"Sellers","DeveloperName":"Sellers"}`)
	readers := f.created("Group", `{"Name":"Readers","DeveloperName":"Readers"}`)
	f.created("GroupMember", `{"GroupId":"`+sellers+`","UserOrGroupId":"`+owner+`"}`)
	f.created("GroupMember", `{"GroupId":"`+readers+`","UserOrGroupId":"`+reader+`"}`)
	setOWD(t, f, "Account", "None")
	acct := mustCreate(t, f, owner, "Account", map[string]any{"Name": "Ruled"})
	if got := queryAs(t, f, reader, "SELECT Name FROM Account").TotalSize; got != 0 {
		t.Fatalf("reader sees %d before the rule", got)
	}
	ctx := context.Background()
	rule, e := f.s.AddSharingRule(ctx, f.tenant, SharingRule{Object: "Account", Name: "Read", Source: "Sellers", Target: readers, Access: "Read"})
	if e != nil {
		t.Fatal(e)
	}
	if got := names(queryAs(t, f, reader, "SELECT Name FROM Account"), "Name"); got != "Ruled" {
		t.Fatalf("reader sees %q", got)
	}
	wantCode(t, "edit through a Read rule", updateAs(t, f, reader, "Account", acct, map[string]any{"Name": "x"}), "INSUFFICIENT_ACCESS_OR_READONLY")
	if e = f.s.RemoveSharingRule(ctx, f.tenant, rule); e != nil {
		t.Fatal(e)
	}
	if _, e = f.s.AddSharingRule(ctx, f.tenant, SharingRule{Object: "Account", Name: "Edit", Source: sellers, Target: "Readers", Access: "Edit"}); e != nil {
		t.Fatal(e)
	}
	if e = updateAs(t, f, reader, "Account", acct, map[string]any{"Name": "Rule edit"}); e != nil {
		t.Fatalf("edit through an Edit rule: %v", e)
	}
	wantCode(t, "delete through a rule", deleteAs(t, f, reader, "Account", acct), "INSUFFICIENT_ACCESS_OR_READONLY")
}

// D34-16, D34-23: Controlled by Parent follows the Account; a contact with
// no Account is private to its owner, not to the owner's managers.
func TestContactControlledByParent(t *testing.T) {
	f := newWriteFixture(t)
	_, mid, rep, peer := hierarchy(t, f)
	setOWD(t, f, "Account", "None")
	acct := mustCreate(t, f, rep, "Account", map[string]any{"Name": "Parent"})
	mustCreate(t, f, rep, "Contact", map[string]any{"LastName": "Child", "AccountId": acct})
	mustCreate(t, f, rep, "Contact", map[string]any{"LastName": "Private"})
	for user, want := range map[string]string{rep: "Child,Private", mid: "Child", peer: "", f.user: "Child,Private"} {
		if got := names(queryAs(t, f, user, "SELECT LastName FROM Contact WHERE LastName IN ('Child','Private')"), "LastName"); got != want {
			t.Errorf("%s sees %q, want %q", user, got, want)
		}
	}
}

// D34-22: with Contact Private, the Account owner reads its contacts, and
// edits them only when their role's ContactAccessForAccountOwner is Edit.
func TestAccountOwnerContactAccess(t *testing.T) {
	f := newWriteFixture(t)
	a, b := sharingUser(t, f, "aoc-a", "Standard User"), sharingUser(t, f, "aoc-b", "Standard User")
	role := f.created("UserRole", `{"Name":"Owner role","DeveloperName":"Owner_role","ContactAccessForAccountOwner":"Read"}`)
	f.do("PATCH", "sobjects/User/"+a, `{"UserRoleId":"`+role+`"}`, false)
	setOWD(t, f, "Contact", "None")
	acct := mustCreate(t, f, a, "Account", map[string]any{"Name": "Owned by a"})
	contact := mustCreate(t, f, b, "Contact", map[string]any{"LastName": "B's", "AccountId": acct})
	if got := queryAs(t, f, a, "SELECT Id FROM Contact WHERE Id = '"+contact+"'").TotalSize; got != 1 {
		t.Fatal("account owner can't read the contact")
	}
	wantCode(t, "edit with Read role setting", updateAs(t, f, a, "Contact", contact, map[string]any{"Title": "x"}), "INSUFFICIENT_ACCESS_OR_READONLY")
	f.do("PATCH", "sobjects/UserRole/"+role, `{"ContactAccessForAccountOwner":"Edit"}`, false)
	if e := updateAs(t, f, a, "Contact", contact, map[string]any{"Title": "Edited"}); e != nil {
		t.Fatalf("edit with Edit role setting: %v", e)
	}
}

// D34-21: reaching a Contact gives Read on its Account.
func TestImplicitParentRead(t *testing.T) {
	f := newWriteFixture(t)
	a, b := sharingUser(t, f, "imp-a", "Standard User"), sharingUser(t, f, "imp-b", "Standard User")
	setOWD(t, f, "Account", "None")
	setOWD(t, f, "Contact", "None")
	acct := mustCreate(t, f, a, "Account", map[string]any{"Name": "Implicit"})
	contact := mustCreate(t, f, a, "Contact", map[string]any{"LastName": "Shared", "AccountId": acct})
	if got := queryAs(t, f, b, "SELECT Name FROM Account").TotalSize; got != 0 {
		t.Fatal("b reads the account before the contact share")
	}
	manualShare(t, f, "Contact", contact, b, "Read")
	if got := names(queryAs(t, f, b, "SELECT Name FROM Account"), "Name"); got != "Implicit" {
		t.Fatalf("implicit parent read: %q", got)
	}
	wantCode(t, "edit an implicitly read account", updateAs(t, f, b, "Account", acct, map[string]any{"Name": "x"}), "INSUFFICIENT_ACCESS_OR_READONLY")
}

// A master-detail detail takes its master's access.
func TestMasterDetailControlledByParent(t *testing.T) {
	f := newWriteFixture(t)
	ctx := context.Background()
	for _, o := range []string{"Master", "Detail"} {
		if _, e := metadata.CreateObject(ctx, f.s.pool, f.tenant, metadata.ObjectInput{Name: o + "__c", Label: o, PluralLabel: o + "s"}); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := metadata.CreateField(ctx, f.s.pool, f.tenant, metadata.FieldInput{Object: "Detail__c", Name: "Master__c", Label: "Master", Type: "MasterDetail", ReferenceTo: "Master__c"}); e != nil {
		t.Fatal(e)
	}
	a, b := sharingUser(t, f, "md-a", "Standard User"), sharingUser(t, f, "md-b", "Standard User")
	set := f.created("PermissionSet", `{"Name":"MD","Label":"MD"}`)
	for _, o := range []string{"Master__c", "Detail__c"} {
		f.created("ObjectPermissions", `{"ParentId":"`+set+`","SobjectType":"`+o+`","PermissionsRead":true,"PermissionsCreate":true,"PermissionsEdit":true}`)
	}
	for _, u := range []string{a, b} {
		f.created("PermissionSetAssignment", `{"AssigneeId":"`+u+`","PermissionSetId":"`+set+`"}`)
	}
	setOWD(t, f, "Master__c", "None")
	master := mustCreate(t, f, a, "Master__c", map[string]any{"Name": "M"})
	mustCreate(t, f, a, "Detail__c", map[string]any{"Name": "D", "Master__c": master})
	if queryAs(t, f, a, "SELECT Name FROM Detail__c").TotalSize != 1 || queryAs(t, f, b, "SELECT Name FROM Detail__c").TotalSize != 0 {
		t.Fatal("detail didn't follow its master")
	}
	manualShare(t, f, "Master__c", master, b, "Read")
	if queryAs(t, f, b, "SELECT Name FROM Detail__c").TotalSize != 1 {
		t.Fatal("detail didn't follow a share on its master")
	}
}
