// Package dbembed exposes the embedded SQL migrations so both the API and the
// standalone migrate command can apply them without shipping loose files.
package dbembed

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
