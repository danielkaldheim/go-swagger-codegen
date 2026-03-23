package swagger

import "strings"

// ResolveRef extracts the definition name from a $ref string like "#/definitions/Account"
func ResolveRef(ref string) string {
	const prefix = "#/definitions/"
	if strings.HasPrefix(ref, prefix) {
		return ref[len(prefix):]
	}
	return ref
}

// ResolveSchema returns the resolved schema for a given schema.
// If the schema has a $ref, it returns the referenced definition.
// Otherwise returns the schema itself.
func ResolveSchema(schema *Schema, definitions map[string]*Schema) *Schema {
	if schema == nil {
		return nil
	}
	if schema.Ref != "" {
		name := ResolveRef(schema.Ref)
		if def, ok := definitions[name]; ok {
			return def
		}
	}
	return schema
}
