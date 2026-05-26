// Package migrations содержит SQL-миграции базы данных, встраиваемые через embed.FS.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
