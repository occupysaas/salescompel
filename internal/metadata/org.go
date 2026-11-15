package metadata

import (
	"context"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrgDefaults are the tenant's default time zone, locale and language, the
// values a new CLI user starts from. An empty value is left unchanged.
type OrgDefaults struct{ TimeZone, Locale, Language string }

// SetOrgDefaults changes the org defaults shown on Organization. Existing
// users keep their own settings (spec "Time zone, locale and language").
func SetOrgDefaults(ctx context.Context, p *pgxpool.Pool, tenant string, d OrgDefaults) error {
	for _, c := range []struct {
		value, name string
		allowed     []string
	}{{d.TimeZone, "time zone", TimeZoneSidKeys}, {d.Locale, "locale", LocaleSidKeys}, {d.Language, "language", LanguageLocaleKeys}} {
		if c.value != "" && !slices.Contains(c.allowed, c.value) {
			return errors.New("unsupported " + c.name + ": " + c.value)
		}
	}
	if d == (OrgDefaults{}) {
		return errors.New("no org defaults given")
	}
	return ownerTx(ctx, p, tenant, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE sc_catalog.settings SET time_zone_sid_key=coalesce(nullif($2,''),time_zone_sid_key),
			default_locale_sid_key=coalesce(nullif($3,''),default_locale_sid_key),language_locale_key=coalesce(nullif($4,''),language_locale_key)
			WHERE tenant_id=$1`, tenant, d.TimeZone, d.Locale, d.Language)
		if e == nil && tag.RowsAffected() != 1 {
			e = errors.New("tenant is not provisioned")
		}
		return e
	})
}
