package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BenchmarkStorageLayout is deliberately opt-in and creates only its own schema.
// Fixed seed 2701: data and operation IDs are deterministic, no customer data.
func BenchmarkStorageLayout(b *testing.B) {
	if os.Getenv("CANDIDATE_DISPOSABLE") != "yes" {
		b.Fatal("CANDIDATE_DISPOSABLE=yes required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		b.Fatal(err)
	}
	defer pool.Close()
	var version string
	pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	b.Log(version)
	for _, fields := range []int{20, 100} {
		for _, skew := range []bool{false, true} {
			for _, layout := range []string{"typed", "jsonb"} {
				b.Run(fmt.Sprintf("%s/fields%d/skew%t", layout, fields, skew), func(b *testing.B) {
					for rep := 0; rep < b.N; rep++ {
						benchLayout(b, ctx, pool, layout, fields, skew)
					}
				})
			}
		}
	}
}
func benchLayout(b *testing.B, ctx context.Context, pool *pgxpool.Pool, layout string, fields int, skew bool) {
	must := func(sql string) {
		b.Helper()
		if _, e := pool.Exec(ctx, sql); e != nil {
			b.Fatal(e)
		}
	}
	must("DROP SCHEMA IF EXISTS sc_benchmark CASCADE; CREATE SCHEMA sc_benchmark")
	defer pool.Exec(ctx, "DROP SCHEMA sc_benchmark CASCADE")
	cols, values := []string{}, []string{}
	for i := 0; i < fields; i++ {
		cols = append(cols, fmt.Sprintf("f%d varchar(80)", i))
		values = append(values, fmt.Sprintf("'value-'||(g%%100)::text"))
	}
	tenant := "((g-1)/10000)::text"
	if skew {
		tenant = "CASE WHEN g<=50000 THEN '0' ELSE (1+(g-50001)%9)::text END"
	}
	data := ""
	value := ""
	projection := "f0"
	field := "f0"
	update := "f1='changed'"
	insertCols := ",f0"
	insertVals := ",'added'"
	if layout == "typed" {
		data = strings.Join(cols, ",")
		value = strings.Join(values, ",")
	} else {
		data = "data jsonb NOT NULL"
		value = fmt.Sprintf("(SELECT jsonb_object_agg('f'||n,'value-'||(g%%100)::text) FROM generate_series(0,%d) n)", fields-1)
		projection = "data->>'f0'"
		field = "(data->>'f0')"
		update = "data=jsonb_set(data,'{f1}','\"changed\"')"
		insertCols = ",data"
		insertVals = ",'{\"f0\":\"added\"}'"
	}
	must("CREATE TABLE sc_benchmark.records(tenant_id text NOT NULL,id bigint NOT NULL,k text," + data + ",PRIMARY KEY(tenant_id,id),UNIQUE(tenant_id,k))")
	must("INSERT INTO sc_benchmark.records SELECT " + tenant + ",g,g::text," + value + " FROM generate_series(1,100000) g")
	must("CREATE INDEX ON sc_benchmark.records(tenant_id,lower(" + field + "),id)")
	must("CREATE TABLE sc_benchmark.children(tenant_id text,id bigint,parent bigint,PRIMARY KEY(tenant_id,id),FOREIGN KEY(tenant_id,parent) REFERENCES sc_benchmark.records(tenant_id,id))")
	must("INSERT INTO sc_benchmark.children SELECT tenant_id,id,id FROM sc_benchmark.records")
	must("ANALYZE sc_benchmark.records; ANALYZE sc_benchmark.children")
	var size int64
	pool.QueryRow(ctx, "SELECT pg_total_relation_size('sc_benchmark.records')").Scan(&size)
	b.Logf("bytes=%d seed=2701", size)
	var plan []byte
	pool.QueryRow(ctx, "EXPLAIN (FORMAT JSON) SELECT id FROM sc_benchmark.records WHERE tenant_id='0' AND lower("+field+")='value-1' ORDER BY id LIMIT 10").Scan(&plan)
	b.Logf("plan=%s", plan)
	ops := map[string]string{
		"read":   "SELECT " + projection + " FROM sc_benchmark.records WHERE tenant_id='0' AND id=$1",
		"update": "UPDATE sc_benchmark.records SET " + update + " WHERE tenant_id='0' AND id=$1",
		"query":  "SELECT id FROM sc_benchmark.records WHERE tenant_id='0' AND lower(" + field + ")='value-1' AND id>$1 ORDER BY id LIMIT 10",
		"join":   "SELECT c.id FROM sc_benchmark.children c JOIN sc_benchmark.records r ON r.tenant_id=c.tenant_id AND r.id=c.parent WHERE c.tenant_id='0' AND c.id=$1",
		"create": "INSERT INTO sc_benchmark.records(tenant_id,id,k" + insertCols + ") VALUES('0',$1,$1::bigint::text" + insertVals + ")",
		"unique": "INSERT INTO sc_benchmark.records(tenant_id,id,k" + insertCols + ") VALUES('0',1000000+$1::bigint,'1'" + insertVals + ") ON CONFLICT(tenant_id,k) DO NOTHING",
	}
	for _, workers := range []int{1, 8} {
		for _, op := range []string{"read", "update", "query", "join", "create", "unique"} {
			// Warmup and measurements use identical SQL; create uses nonoverlapping IDs.
			var wg sync.WaitGroup
			var mu sync.Mutex
			durations := []int64{}
			errs := []error{}
			start := time.Now()
			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(ctx context.Context, w int) {
					defer wg.Done()
					for n := 0; n < 110; n++ {
						id := int64(1 + (n*97+w*13)%9000)
						if op == "create" {
							id = int64(200000 + workers*10000 + w*110 + n)
						}
						at := time.Now()
						_, e := pool.Exec(ctx, ops[op], id)
						elapsed := time.Since(at).Microseconds()
						mu.Lock()
						if e != nil {
							errs = append(errs, e)
						}
						if n >= 10 {
							durations = append(durations, elapsed)
						}
						mu.Unlock()
					}
				}(ctx, w)
			}
			wg.Wait()
			if len(errs) > 0 {
				b.Fatal(op, errs[0])
			}
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			row := map[string]any{"operation": op, "workers": workers, "p50_us": durations[len(durations)/2], "p95_us": durations[len(durations)*95/100], "ops_s": float64(workers*110) / time.Since(start).Seconds(), "errors": len(errs)}
			encoded, _ := json.Marshal(row)
			b.Log(string(encoded))
		}
	}
	must("CREATE TABLE sc_benchmark.field_definitions(name text PRIMARY KEY,length integer NOT NULL)")
	start := time.Now()
	tx, e := pool.Begin(ctx)
	if e != nil {
		b.Fatal(e)
	}
	if layout == "typed" {
		if _, e = tx.Exec(ctx, "ALTER TABLE sc_benchmark.records ADD COLUMN extra varchar(80)"); e != nil {
			tx.Rollback(ctx)
			b.Fatal(e)
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO sc_benchmark.field_definitions VALUES('extra',80)"); e != nil {
		tx.Rollback(ctx)
		b.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		b.Fatal(e)
	}
	b.Logf("field_add_us=%d", time.Since(start).Microseconds())
	// Every loaded child has a same-tenant parent; duplicate and cross-tenant FK are enforced in SQL.
	var count int
	if e := pool.QueryRow(ctx, "SELECT count(*) FROM sc_benchmark.records WHERE tenant_id='0'").Scan(&count); e != nil || count < 10000 {
		b.Fatal("tenant rows", count, e)
	}
}
