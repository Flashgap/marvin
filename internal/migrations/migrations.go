// Package migrations exposes the Atlas-managed SQL migrations as an embedded
// filesystem. Each supported driver has its own subdirectory; consumers
// (typically pkg/database via Client.Migrate) select the right subdir by
// driver name.
//
// To author a new migration, install the Atlas CLI and run
// `make migrate-diff driver=<postgres|mysql> name=<short_description>` from the repository root.
package migrations

import "embed"

//go:embed postgres/* mysql/*
var FS embed.FS

