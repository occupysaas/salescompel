package soql

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/contract"
	"salescompel/internal/metadata"
	"salescompel/internal/policy"
)

var locatorPathPattern = regexp.MustCompile(`^(01g[A-Za-z0-9]{15})-([0-9]{1,9})$`)

// invalidLocator is the closed-vocabulary locator error. It never names the
// query or a field. spec: soql-06
func invalidLocator() error {
	return contract.Invalid("INVALID_QUERY_LOCATOR", "Query locator is invalid")
}

// ParseLocatorPath splits a <locator>-<offset> path. Anything else is
// INVALID_QUERY_LOCATOR. spec: page-01
func ParseLocatorPath(path string) (string, int, error) {
	m := locatorPathPattern.FindStringSubmatch(path)
	if m == nil {
		return "", 0, invalidLocator()
	}
	offset, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, invalidLocator()
	}
	return m[1], offset, nil
}

// NextPage serves one page of a stored locator. The canonical query text is
// re-parsed and re-bound against the current catalog, so a removed or denied
// selected field fails without leaking it. spec: page-01
func NextPage(ctx context.Context, tx pgx.Tx, exec policy.ExecutionContext, snap metadata.Snapshot, locator string, offset int, batch int) (Result, error) {
	if offset < 0 {
		return Result{}, invalidLocator()
	}
	batch = clampBatch(batch)
	var userID, digest string
	var raw []byte
	var ids []string
	var expires time.Time
	err := tx.QueryRow(ctx,
		`SELECT user_id,query_sha256,plan,ids,expires_at FROM sc_query.locators WHERE tenant_id=$1 AND id=$2`,
		exec.Principal.Tenant, locator).Scan(&userID, &digest, &raw, &ids, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, invalidLocator()
	}
	if err != nil {
		return Result{}, dbErr(err)
	}
	if userID != exec.Principal.User {
		return Result{}, invalidLocator()
	}
	now := time.Now()
	if !expires.After(now) {
		// Encountered expired rows are removed; callers that commit keep it.
		_, _ = tx.Exec(ctx, "DELETE FROM sc_query.locators WHERE tenant_id=$1 AND id=$2", exec.Principal.Tenant, locator)
		return Result{}, invalidLocator()
	}
	var stored locatorPlan
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Query == "" {
		return Result{}, invalidLocator()
	}
	sum := sha256.Sum256([]byte(stored.Query))
	if hex.EncodeToString(sum[:]) != digest {
		return Result{}, invalidLocator()
	}
	q, err := Parse(stored.Query)
	if err != nil {
		return Result{}, invalidLocator()
	}
	bound, err := Bind(q, snap, exec.Principal, func() time.Time { return now })
	if err != nil {
		return Result{}, invalidLocator()
	}
	if bound.Aggregate || bound.CountOnly || bound.Offset != nil {
		return Result{}, invalidLocator()
	}
	if offset >= len(ids) {
		return Result{}, invalidLocator()
	}
	hi := offset + batch
	if hi > len(ids) {
		hi = len(ids)
	}
	records, err := pageRows(ctx, tx, bound, ids[offset:hi])
	if err != nil {
		return Result{}, err
	}
	done := hi >= len(ids)
	next := ""
	if !done {
		next = locator + "-" + strconv.Itoa(hi)
	}
	return Result{TotalSize: len(ids), Done: done, Records: records, NextLocator: next}, nil
}
