package records

import (
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
	"salescompel/internal/recordid"
	"salescompel/internal/soql"
)

// Manual shares through AccountShare and ContactShare (D34-04..08). Only
// the owner, users above the owner and Modify All holders share a record;
// only Manual rows are written; Owner rows are computed by the share views.

type shareShape struct {
	parentField, levelField string
	children                []string // AccountShare's child access fields
}

var shareShapes = map[string]shareShape{
	"AccountShare": {"AccountId", "AccountAccessLevel", []string{"ContactAccessLevel", "OpportunityAccessLevel", "CaseAccessLevel"}},
	"ContactShare": {"ContactId", "ContactAccessLevel", nil},
}

var childColumns = map[string]string{"ContactAccessLevel": "contact_access_level", "OpportunityAccessLevel": "opportunity_access_level", "CaseAccessLevel": "case_access_level"}

// shareWrite applies the share rules to a create, update or delete.
func shareWrite(w setupWriter, op, object, id string, data map[string]any) (string, error) {
	shape := shareShapes[object]
	parentName, _ := metadata.ShareParent(object)
	parent, err := w.p.Catalog.Object(parentName)
	if err != nil {
		return "", err
	}
	if op == "create" {
		return w.createShare(object, shape, parent, data)
	}
	var record string
	err = w.db.QueryRow(w.ctx, "SELECT record_id FROM sc_sharing.shares WHERE tenant_id=$1 AND id=$2", w.p.Tenant, id).Scan(&record)
	if err == pgx.ErrNoRows {
		// A computed Owner row can't be changed; anything else is unknown.
		var computed bool
		if qerr := w.db.QueryRow(w.ctx, "SELECT EXISTS(SELECT FROM sc_setup."+tableOf(object)+" WHERE tenant_id=$1 AND id=$2)", w.p.Tenant, id).Scan(&computed); qerr == nil && computed {
			return "", errInsufficientAccess
		}
		return "", dbError(err)
	}
	if err != nil {
		return "", dbError(err)
	}
	if err = requireAccess(w.ctx, w.db, w.p, parent, record, soql.LevelDelete); err != nil {
		return "", err
	}
	if op == "delete" {
		_, err = w.db.Exec(w.ctx, "DELETE FROM sc_sharing.shares WHERE tenant_id=$1 AND id=$2", w.p.Tenant, id)
		return id, dbError(err)
	}
	if err = Validate(w.p, object, data, false); err != nil {
		return "", err
	}
	sets, args, err := w.shareLevels(shape, parent, data, false)
	if err != nil || len(sets) == 0 {
		return id, err
	}
	args = append(args, w.p.Tenant, id)
	_, err = w.db.Exec(w.ctx, "UPDATE sc_sharing.shares SET "+strings.Join(sets, ",")+" WHERE tenant_id=$"+itoa(len(args)-1)+" AND id=$"+itoa(len(args)), args...)
	return id, dbError(err)
}

func tableOf(object string) string {
	if object == "AccountShare" {
		return "account_shares"
	}
	return "contact_shares"
}

func (w setupWriter) createShare(object string, shape shareShape, parent Object, data map[string]any) (string, error) {
	if v, ok := data["RowCause"]; ok && v != nil && v != "Manual" {
		return "", fieldProblem(integrityCode, "Only Manual shares can be created.", "RowCause")
	}
	data["RowCause"] = "Manual"
	if err := Validate(w.p, object, data, true); err != nil {
		return "", err
	}
	raw, _ := data[shape.parentField].(string)
	record, err := normalizeID(w.p, parent.Name, raw)
	if err != nil {
		return "", malformedID(shape.parentField, raw)
	}
	if read, _, err := recordAccess(w.ctx, w.db, w.p, parent, record, soql.LevelRead); err != nil {
		return "", err
	} else if !read {
		return "", crossReference(shape.parentField)
	}
	if err = requireAccess(w.ctx, w.db, w.p, parent, record, soql.LevelDelete); err != nil {
		return "", err // D34-04, D34-05
	}
	grantee, _ := data["UserOrGroupId"].(string)
	if grantee, err = w.memberID(grantee); err != nil {
		return "", err
	}
	sets, args, err := w.shareLevels(shape, parent, data, true)
	if err != nil {
		return "", err
	}
	id, err := recordid.New(prefixOf(w, object))
	if err != nil {
		return "", dbError(err)
	}
	cols := []string{"tenant_id", "id", "object_id", "record_id", "user_or_group_id"}
	vals := []any{w.p.Tenant, id, parent.ID, record, grantee}
	updates := []string{}
	for i, s := range sets {
		col := s[:strings.Index(s, "=")]
		cols = append(cols, col)
		vals = append(vals, args[i])
		updates = append(updates, col+"=excluded."+col)
	}
	marks := make([]string, len(vals))
	for i := range vals {
		marks[i] = "$" + itoa(i+1)
	}
	// Sharing a record again with the same grantee updates that share
	// (D34-06).
	err = w.db.QueryRow(w.ctx, "INSERT INTO sc_sharing.shares("+strings.Join(cols, ",")+") VALUES("+strings.Join(marks, ",")+
		") ON CONFLICT(tenant_id,record_id,user_or_group_id,row_cause) DO UPDATE SET "+strings.Join(updates, ",")+" RETURNING id", vals...).Scan(&id)
	return id, dbError(err)
}

func prefixOf(w setupWriter, object string) string {
	o, err := w.p.Catalog.Object(object)
	if err != nil {
		return "00r"
	}
	return o.Prefix
}

// shareLevels validates the level fields: the record level is Read or Edit
// and more open than the org-wide default; child levels are None, Read or
// Edit (D34-06). required demands the record level.
func (w setupWriter) shareLevels(shape shareShape, parent Object, data map[string]any, required bool) ([]string, []any, error) {
	var sets []string
	var args []any
	v, ok := data[shape.levelField]
	if ok && v != nil {
		level, _ := v.(string)
		owd := w.p.Sharing.Access(parent)
		if level != "Read" && level != "Edit" || owd == metadata.AccessRead && level == "Read" || owd == metadata.AccessEdit {
			return nil, nil, fieldProblem(integrityCode, "Access level must be more permissive than the organization-wide default.", shape.levelField)
		}
		sets, args = append(sets, "access_level=$"+itoa(len(args)+1)), append(args, level)
	} else if required {
		return nil, nil, fieldProblem("REQUIRED_FIELD_MISSING", "Required fields are missing: ["+shape.levelField+"]", shape.levelField)
	}
	for _, f := range shape.children {
		if v, ok := data[f]; ok {
			sets, args = append(sets, childColumns[f]+"=$"+itoa(len(args)+1)), append(args, v)
		}
	}
	return sets, args, nil
}

func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return digits[n : n+1]
	}
	return itoa(n/10) + digits[n%10:n%10+1]
}
