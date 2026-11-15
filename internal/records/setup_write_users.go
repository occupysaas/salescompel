package records

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"salescompel/internal/fieldtype"
	"salescompel/internal/recordid"
)

// userRequired are the fields a User insert must supply; nothing is filled
// from the org defaults over the API (D33-16).
var userRequired = []string{"Username", "LastName", "Alias", "Email", "ProfileId", "TimeZoneSidKey", "LocaleSidKey", "LanguageLocaleKey", "EmailEncodingKey"}

// user writes a User. A create makes the credential row (no usable
// password) through sc_setup.create_identity and the system-maintained
// assignment to the profile's owned set; a Username change goes through
// sc_setup.rename_identity; a ProfileId change moves that assignment. Users
// are never deleted (D33-31; writeAccess refuses it from the describe flag).
func (w setupWriter) user(op, id string, data map[string]any) (string, error) {
	switch op {
	case "create":
		return w.createUser(data)
	case "update":
		return id, w.updateUser(id, data)
	}
	return "", errInsufficientAccess // D33-31
}

func (w setupWriter) createUser(data map[string]any) (string, error) {
	var missing []string
	for _, name := range userRequired {
		if v, ok := data[name]; !ok || v == nil || v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// D33-16
		return "", &APIError{Status: 400, Code: "REQUIRED_FIELD_MISSING", Message: "Required fields are missing: [" + strings.Join(missing, ", ") + "]", Fields: missing}
	}
	username, err := checkUsername(data["Username"])
	if err != nil {
		return "", err
	}
	if err = Validate(w.p, "User", data, true); err != nil {
		return "", err
	}
	profile, err := w.reference(data, "ProfileId", "Profile")
	if err != nil {
		return "", err
	}
	id, err := recordid.New("005")
	if err != nil {
		return "", dbError(err)
	}
	if _, err = w.db.Exec(w.ctx, "SELECT sc_setup.create_identity($1,$2,$3)", id, w.p.Tenant, username); err != nil {
		return "", identityError(err)
	}
	if _, err = insertRecord(w.ctx, w.db, w.p, "User", data, id, map[string]any{"username_canonical": strings.ToLower(username)}); err != nil {
		return "", identityError(err)
	}
	assignment, err := recordid.New("0Pa")
	if err != nil {
		return "", dbError(err)
	}
	_, err = w.db.Exec(w.ctx, `INSERT INTO sc_setup.permission_set_assignments(id,tenant_id,assignee_id,permission_set_id)
		SELECT $1,tenant_id,$3,permission_set_id FROM sc_setup.profiles WHERE tenant_id=$2 AND id=$4`, assignment, w.p.Tenant, id, profile)
	return id, dbError(err)
}

func (w setupWriter) updateUser(id string, data map[string]any) error {
	var oldProfile string
	err := w.db.QueryRow(w.ctx, "SELECT profile_id FROM sc_setup.users WHERE tenant_id=$1 AND id=$2 FOR UPDATE", w.p.Tenant, id).Scan(&oldProfile)
	if err != nil {
		return dbError(err)
	}
	if err = Validate(w.p, "User", data, false); err != nil {
		return err
	}
	extra := map[string]any{}
	if v, ok := data["Username"]; ok {
		username, err := checkUsername(v)
		if err != nil {
			return err
		}
		if _, err = w.db.Exec(w.ctx, "SELECT sc_setup.rename_identity($1,$2,$3)", id, w.p.Tenant, username); err != nil {
			return identityError(err)
		}
		extra["username_canonical"] = strings.ToLower(username)
	}
	profile, err := w.reference(data, "ProfileId", "Profile")
	if err != nil {
		return err
	}
	if err = updateRecord(w.ctx, w.db, w.p, "User", id, data, extra); err != nil {
		return identityError(err)
	}
	if profile == "" || profile == oldProfile {
		return nil
	}
	// The system-maintained assignment follows the profile (D33-21).
	_, err = w.db.Exec(w.ctx, `UPDATE sc_setup.permission_set_assignments a SET permission_set_id=np.permission_set_id
		FROM sc_setup.profiles op, sc_setup.profiles np
		WHERE op.tenant_id=$1 AND op.id=$3 AND np.tenant_id=$1 AND np.id=$4
		AND a.tenant_id=$1 AND a.assignee_id=$2 AND a.permission_set_id=op.permission_set_id`, w.p.Tenant, id, oldProfile, profile)
	return dbError(err)
}

// checkUsername requires an email address within the credential row's
// rules: ASCII and at most 80 bytes (D33-17).
func checkUsername(v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fieldProblem("JSON_PARSER_ERROR", "Expected a string or null", "Username")
	}
	if len(s) > 80 {
		return "", fieldProblem("STRING_TOO_LONG", "Invalid or oversized field value", "Username")
	}
	if !fieldtype.ValidEmail(s) || len(s) != len([]rune(s)) {
		return "", fieldProblem("INVALID_EMAIL_ADDRESS", "Username: invalid email address: "+s, "Username")
	}
	return s, nil
}

// identityError maps the identity functions' and the users table's
// failures: a username taken in any tenant is DUPLICATE_USERNAME (D33-15).
func identityError(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" && strings.Contains(pe.ConstraintName, "username_canonical") {
		return fieldProblem("DUPLICATE_USERNAME", "Duplicate Username.", "Username")
	}
	if errors.As(err, &pe) && pe.Code == "22023" {
		return fieldProblem("INVALID_EMAIL_ADDRESS", "Username: invalid email address", "Username")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return dbError(err)
	}
	return dbErrorPreserve(err)
}
