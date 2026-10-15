# Migration handoff from Plan 1

The portable bundle and migration procedure remain specified in the roadmap. Plan 1 produces contracts for Account/Contact relationships, external IDs, metadata and independent user locale settings that a later importer must preserve. It does not implement or certify export/import.

The first migration rehearsal must export fixture data plus field metadata and user-setting mappings, import into an isolated SalesCompel tenant, then rerun this laboratory against the candidate. Compare persisted values, relationship identity, permissions, language/locale and metadata; record exact schema/API versions. Keep audit provenance, custom translations and credentials separate. A CSV-only success is not an organization migration.
