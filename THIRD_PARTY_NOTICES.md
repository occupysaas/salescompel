# Third-party notices

SalesCompel code and documentation are Apache-2.0 licensed unless a file states otherwise.

The unmodified `compat/fixtures/openapi/salesforce/einstein-bot-v5.yaml` comes from Salesforce's `forcedotcom/einstein-bot-openapi-codegen-maven-plugin` repository. Copyright (c) 2022, Salesforce.com, Inc. All rights reserved. Its BSD 3-Clause license is retained alongside the fixture as `LICENSE.txt`; `provenance.json` pins the source commit, URL and SHA-256. This inclusion does not imply Salesforce endorsement.

Installed npm and Python packages retain their respective licenses. They are test tooling; package versions are recorded in their lockfiles and their license texts remain in installed package distributions. No Salesforce product implementation or branding assets are included.

The Go dependency `golang.org/x/crypto` is pinned to v0.42.0
(https://pkg.go.dev/golang.org/x/crypto@v0.42.0), used for Argon2id password
hashing in `internal/platform/oauth_secrets.go`. It is distributed under the
BSD 3-Clause license; copyright The Go Authors. Its original license and
copyright notices remain in the module distribution. No upstream source is
copied or modified in this repository.

Its required indirect Go dependency `golang.org/x/sys` is pinned to v0.36.0
(https://pkg.go.dev/golang.org/x/sys@v0.36.0), under the BSD 3-Clause license;
copyright The Go Authors. Original notices remain in the module distribution.
