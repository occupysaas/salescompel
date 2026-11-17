package soql

import (
	"strings"
	"testing"

	"salescompel/internal/metadata"
	"salescompel/internal/policy"
	"salescompel/internal/policy/policytest"
)

func sharingPrincipal(perms policy.Permissions, owd map[string]string) *policy.Principal {
	snap := metadata.Snapshot{Objects: metadata.StandardObjects()}
	for i := range snap.Objects {
		snap.Objects[i].ID = "obj-" + snap.Objects[i].Name
	}
	return &policy.Principal{Tenant: "t", User: "005u", Perms: perms, Catalog: &snap, Sharing: policy.NewSharing(owd)}
}

func crudPerms() policy.Permissions {
	crud := policy.ObjectAccess{Read: true, Create: true, Edit: true, Delete: true}
	return policy.Union(policy.PermissionSet{Objects: map[string]policy.ObjectAccess{"Account": crud, "Contact": crud}})
}

func clauseFor(p *policy.Principal, object string, level Level) (string, []any) {
	var args []any
	o, _ := p.Catalog.Object(object)
	c := SharingClause(p, "r", o, level, func(v any) int { args = append(args, v); return len(args) + 1 })
	return c, args
}

// Each term of the spec's predicate, per level (Task 6).
func TestSharingClauseTerms(t *testing.T) {
	std := crudPerms()
	cases := []struct {
		name   string
		perms  policy.Permissions
		owd    map[string]string
		object string
		level  Level
		want   []string // substrings; nil means no clause
		absent []string
	}{
		{"public read/write read", std, nil, "Account", LevelRead, nil, nil},
		{"public read/write edit", std, nil, "Account", LevelEdit, nil, nil},
		{"public read/write delete needs owner", std, nil, "Account", LevelDelete, []string{"r.owner_id IN"}, []string{"sc_sharing.shares", "owner_rules"}},
		{"read only read", std, map[string]string{"Account": "Read"}, "Account", LevelRead, nil, nil},
		{"read only edit", std, map[string]string{"Account": "Read"}, "Account", LevelEdit, []string{"r.owner_id IN", "sh.access_level IN ('Edit')", "owner_rules"}, nil},
		{"private read", std, map[string]string{"Account": "None"}, "Account", LevelRead, []string{"r.owner_id IN", "sh.access_level IN ('Read','Edit')", "owner_rules"}, []string{"ic."}},
		{"private read with private contacts implies parent", std, map[string]string{"Account": "None", "Contact": "None"}, "Account", LevelRead, []string{"FROM \"public\".\"contacts\" ic"}, nil},
		{"view all data bypass", policytest.Admin(), map[string]string{"Account": "None"}, "Account", LevelDelete, nil, nil},
		{"unknown OWD fails closed", std, map[string]string{"Account": "Transfer"}, "Account", LevelRead, []string{"FALSE"}, nil},
		{"contact follows a private account", std, map[string]string{"Account": "None"}, "Contact", LevelEdit, []string{"EXISTS(SELECT 1 FROM \"public\".\"accounts\" sp1", "r.\"account_id\" IS NULL AND r.owner_id="}, nil},
		{"contact under public account", std, nil, "Contact", LevelRead, []string{"r.\"account_id\" IS NOT NULL OR"}, nil},
		{"private contact own OWD", std, map[string]string{"Account": "None", "Contact": "None"}, "Contact", LevelEdit, []string{"contact_access_for_account_owner='Edit'", "cs.contact_access_level IN ('Edit')"}, nil},
		{"private contact read honours None", std, map[string]string{"Account": "None", "Contact": "None"}, "Contact", LevelRead, []string{"coalesce(ro.contact_access_for_account_owner,'Read')<>'None'"}, nil},
		{"share rows follow the parent", std, map[string]string{"Account": "None"}, "AccountShare", LevelRead, []string{"EXISTS(SELECT 1 FROM \"public\".\"accounts\" sp1"}, nil},
		{"setup objects unaffected", std, map[string]string{"Account": "None"}, "User", LevelRead, nil, nil},
	}
	for _, c := range cases {
		got, _ := clauseFor(sharingPrincipal(c.perms, c.owd), c.object, c.level)
		if c.want == nil && got != "" {
			t.Errorf("%s: want no clause, got %s", c.name, got)
		}
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q in %s", c.name, w, got)
			}
		}
		for _, w := range c.absent {
			if strings.Contains(got, w) {
				t.Errorf("%s: unexpected %q in %s", c.name, w, got)
			}
		}
	}
}

// The user is one parameter, numbered after the caller's; the rule term
// adds the object ID.
func TestSharingClauseArgs(t *testing.T) {
	p := sharingPrincipal(crudPerms(), map[string]string{"Account": "None"})
	got, args := clauseFor(p, "Account", LevelRead)
	if len(args) != 3 || args[0] != "t" || args[1] != "005u" || args[2] != "obj-Account" || !strings.Contains(got, "up.tenant_id=$2 AND up.user_id=$3") || !strings.Contains(got, "ru.object_id=$4") {
		t.Fatalf("args %v in %s", args, got)
	}
}
