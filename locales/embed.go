package locales

import "embed"

// Messages contains the shared browser and server message catalogs.
//
//go:embed messages/*.json
var Messages embed.FS
