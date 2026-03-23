package generator

import "embed"

//go:embed templates/dart/*.go.tmpl
//go:embed templates/dart/auth/*.go.tmpl
var templateFS embed.FS
