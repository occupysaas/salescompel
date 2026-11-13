package soql

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"salescompel/internal/metadata"
)

// maxLocatorIDs bounds one stored membership; larger results fail rather
// than page. spec: page-01
const maxLocatorIDs = 50_000

// pageArg reserves the stored-ID array parameter for a locator page.
func (c *compiler) pageArg(b *Bound) int {
	if b.PageIDs == nil {
		return 0
	}
	return c.addArg(b.PageIDs)
}

// executePaged fixes membership on the first page and serves it in batches.
// A result that fits one batch stores no locator. spec: page-01
func executePaged(ctx context.Context, tx pgx.Tx, bound *Bound, plan *Plan, batch int, user string) (Result, error) {
	ids, err := membershipIDs(ctx, tx, bound)
	if err != nil {
		return Result{}, err
	}
	if len(ids) > maxLocatorIDs {
		return Result{}, tooComplicated("query result exceeds the locator limit; narrow the query")
	}
	total := len(ids)
	var locator string
	if total > batch {
		locator, err = storeLocator(ctx, tx, bound, ids, user)
		if err != nil {
			return Result{}, err
		}
	}
	hi := batch
	if hi > total {
		hi = total
	}
	records, err := pageRows(ctx, tx, bound, ids[:hi])
	if err != nil {
		return Result{}, err
	}
	done := hi >= total
	next := ""
	if !done {
		next = locator + "-" + strconv.Itoa(hi)
	}
	return Result{TotalSize: total, Done: done, Records: records, NextLocator: next}, nil
}

// membershipIDs runs an ID-only ordered statement for the query's whole
// membership, one row past the locator cap so overflow is detectable.
func membershipIDs(ctx context.Context, tx pgx.Tx, bound *Bound) ([]string, error) {
	idField, err := idFieldOf(bound.Object)
	if err != nil {
		return nil, err
	}
	m := *bound
	m.Columns = []BoundColumn{{Key: "Id", Field: idField}}
	m.Children = nil
	m.PageIDs = nil
	plan, err := Compile(&m)
	if err != nil {
		return nil, err
	}
	limit := maxLocatorIDs + 1
	if bound.Limit != nil && *bound.Limit < limit {
		limit = *bound.Limit
	}
	plan.Stmt.Args[plan.LimitArg] = limit
	rows, err := tx.Query(ctx, plan.Stmt.SQL, plan.Stmt.Args...)
	if err != nil {
		return nil, dbErr(err)
	}
	defer rows.Close()
	ids := make([]string, 0, 64)
	for rows.Next() {
		vals, e := rows.Values()
		if e != nil {
			return nil, dbErr(e)
		}
		id, ok := vals[0].(string)
		if !ok {
			return nil, malformed("query id is unavailable")
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, dbErr(err)
	}
	return ids, nil
}

// pageRows loads the page's current field values in stored-ID order.
func pageRows(ctx context.Context, tx pgx.Tx, bound *Bound, ids []string) ([]map[string]any, error) {
	page := *bound
	page.PageIDs = ids
	plan, err := Compile(&page)
	if err != nil {
		return nil, err
	}
	plan.Stmt.Args[plan.LimitArg] = len(ids)
	return runRows(ctx, tx, &page, plan)
}

// executeAggregate runs a grouped query; aggregates are never paged.
func executeAggregate(ctx context.Context, tx pgx.Tx, bound *Bound, plan *Plan) (Result, error) {
	fetch := maxAggregateGroups + 1
	if bound.Limit != nil && *bound.Limit <= maxAggregateGroups {
		fetch = *bound.Limit
	}
	plan.Stmt.Args[plan.LimitArg] = fetch
	records, err := runRows(ctx, tx, bound, plan)
	if err != nil {
		return Result{}, err
	}
	if len(records) > maxAggregateGroups {
		return Result{}, tooComplicated("aggregate query exceeds the group limit; narrow the query")
	}
	return Result{TotalSize: len(records), Done: true, Records: records}, nil
}

// locatorPlan is the versioned, re-validated form stored in plan. It holds
// canonical query text, never SQL to replay. spec: page-01
type locatorPlan struct {
	Query    string `json:"query"`
	Revision int64  `json:"revision"`
}

// storeLocator inserts a locator and cleans up expired and over-limit rows in
// the same transaction. spec: page-01
func storeLocator(ctx context.Context, tx pgx.Tx, bound *Bound, ids []string, user string) (string, error) {
	if bound.Source == "" {
		return "", malformed("query plan is unavailable")
	}
	id, err := newLocatorID()
	if err != nil {
		return "", dbErr(err)
	}
	payload, err := json.Marshal(locatorPlan{Query: bound.Source, Revision: bound.Revision})
	if err != nil {
		return "", dbErr(err)
	}
	sum := sha256.Sum256([]byte(bound.Source))
	tenant := bound.Object.Tenant
	if _, err = tx.Exec(ctx, "DELETE FROM sc_query.locators WHERE tenant_id=$1 AND expires_at <= now()", tenant); err != nil {
		return "", dbErr(err)
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO sc_query.locators(tenant_id,id,user_id,query_sha256,plan,ids,created_at,expires_at)
		 VALUES($1,$2,$3,$4,$5::jsonb,$6,now(),now()+interval '2 days')`,
		tenant, id, user, hex.EncodeToString(sum[:]), string(payload), ids); err != nil {
		return "", dbErr(err)
	}
	// At most 10 open locators per user; opening an eleventh evicts the oldest.
	if _, err = tx.Exec(ctx,
		`DELETE FROM sc_query.locators WHERE tenant_id=$1 AND id IN (
		   SELECT id FROM sc_query.locators WHERE tenant_id=$1 AND user_id=$2
		   ORDER BY created_at DESC, id DESC OFFSET 10)`, tenant, user); err != nil {
		return "", dbErr(err)
	}
	return id, nil
}

func idFieldOf(object metadata.Object) (metadata.Field, error) {
	for _, f := range object.Fields {
		if strings.EqualFold(f.Name, "Id") {
			return f, nil
		}
	}
	return metadata.Field{}, malformed("object has no id field")
}

const locatorAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// newLocatorID returns "01g" plus 15 random base62 characters.
func newLocatorID() (string, error) {
	out := make([]byte, 3, 18)
	copy(out, "01g")
	var buf [32]byte
	for len(out) < 18 {
		if _, err := rand.Read(buf[:]); err != nil {
			return "", err
		}
		for _, x := range buf {
			// Reject the biased tail so each character is uniform base62.
			if x >= 248 {
				continue
			}
			out = append(out, locatorAlphabet[int(x)%62])
			if len(out) == 18 {
				break
			}
		}
	}
	return string(out), nil
}
