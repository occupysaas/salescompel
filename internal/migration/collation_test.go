package migration

import (
	"context"
	"strings"
	"testing"
)

// The checksum of migration 1 covers the frozen legacy adoption code
// (internal/migration/legacy and v1.go). Released installs record it, so it
// must never change; collation handling lives outside that set (issue #43).
const baselineChecksum = "96ec316f4a7a67c7c8707ada9ad93a0d3037a741aaa89fb332f2d7247ab8ecdb"

func TestBaselineChecksumUnchanged(t *testing.T) {
	if got := manifest()[0].entry.Checksum; got != baselineChecksum {
		t.Fatalf("migration 1 checksum changed to %s; frozen legacy code was edited", got)
	}
}

func TestByteOrderCollationRefused(t *testing.T) {
	for _, tc := range []struct{ name, options string }{
		{"libc C", "TEMPLATE template0 ENCODING 'UTF8' LOCALE 'C'"},
		{"libc POSIX", "TEMPLATE template0 ENCODING 'UTF8' LOCALE 'POSIX'"},
		{"builtin C.UTF-8", "TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER builtin BUILTIN_LOCALE 'C.UTF-8' LOCALE 'C'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPoolWith(t, tc.options)
			ctx := context.Background()
			e := Up(ctx, p)
			if e == nil || !strings.Contains(e.Error(), "collation") || !strings.Contains(e.Error(), "no upgrade was applied") {
				t.Fatalf("want a collation error, got %v", e)
			}
			var tables int
			if e = p.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname IN ('public','sc_control')").Scan(&tables); e != nil || tables != 0 {
				t.Fatalf("refused migration left %d tables (%v)", tables, e)
			}
		})
	}
}

func TestLinguisticCollationMigrates(t *testing.T) {
	p := testPoolWith(t, "TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER icu ICU_LOCALE 'en-US' LOCALE 'C'")
	if e := Up(context.Background(), p); e != nil {
		t.Fatal(e)
	}
}
