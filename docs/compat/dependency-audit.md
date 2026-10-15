# Connector toolchain audit — 2026-10-15

The clean locked npm install succeeds. `npm audit --json` reports **41 affected package entries: 13 moderate, 28 high, 0 critical**. The direct affected dependency is Salesforce CLI; it and its bundled/transitive tools remain subject to upstream advisories. These counts are npm package entries, not a count of independent exploitable vulnerabilities in SalesCompel.

The CLI was updated to 2.156.0 and unused duplicate direct plugin installations were removed. Its actual bundled plugin versions are recorded in capture reports. A blanket audit fix encountered a bundled optional-platform package error; the subsequent clean install from the final lockfile succeeded. The remaining CLI recommendation from npm involves a major downgrade, which was not applied blindly because it would change the tested command contracts.

No claim of an audit-clean dependency tree is made. The toolchain is used in the isolated compatibility laboratory, not shipped as part of a Go server. Keep reference tests on disposable data and track upstream fixes before expanding their use. Rerun `npm audit` when changing the lockfile. The final Python environment passes `pip check`; no Python security-audit result is claimed.
