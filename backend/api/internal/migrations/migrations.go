// Package migrations embeds the SQL migration files (PDR.md section 5 ER
// diagram) so they ship inside the compiled binary.
package migrations

import "embed"

//go:embed sql/*.sql
var FS embed.FS
