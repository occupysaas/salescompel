package platform

import (
	"net/http"
	"strconv"
	"strings"
)

// queryBatchSize parses Sforce-Query-Options: batchSize=n and clamps it to
// the documented 200–2,000 range. spec: page-01
func queryBatchSize(r *http.Request) int {
	for _, part := range strings.FieldsFunc(r.Header.Get("Sforce-Query-Options"), func(c rune) bool {
		return c == ',' || c == ';'
	}) {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 || !strings.EqualFold(strings.TrimSpace(kv[0]), "batchSize") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil {
			return 2000
		}
		if n < 200 {
			return 200
		}
		if n > 2000 {
			return 2000
		}
		return n
	}
	return 2000
}
