package generator

import (
	"strings"
	"text/template"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
)

func buildFuncMap() template.FuncMap {
	return template.FuncMap{
		"lowercase": strings.ToLower,
		"uppercase": strings.ToUpper,
		"titlecase": strings.Title,
		"camelize": func(s string) string {
			return codegen.Camelize(s, false)
		},
		"camelizeLower": func(s string) string {
			return codegen.Camelize(s, true)
		},
		"underscore": codegen.Underscore,
		"not": func(b bool) bool {
			return !b
		},
		"and": func(a, b bool) bool {
			return a && b
		},
		"or": func(a, b bool) bool {
			return a || b
		},
		"eq": func(a, b string) bool {
			return a == b
		},
		"escapeText":           escapeText,
		"renderOperations":     renderOperations,
		"renderVarDecls":      renderVarDeclarations,
		"renderCtorParams":    renderConstructorParams,
		"renderToStringVars":  renderToStringVars,
		"renderFromJsonBody":  renderFromJsonBody,
		"renderToJsonBody":    renderToJsonBody,
		"renderCopyWithParams":  renderCopyWithParams,
		"renderCopyWithArgs":   renderCopyWithArgs,
		"renderReadmeApiTable": renderReadmeApiTable,
		"renderReadmeModelList": renderReadmeModelList,
		"renderReadmeAuth":     renderReadmeAuth,
	}
}
