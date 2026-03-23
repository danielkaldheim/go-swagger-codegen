package codegen

import (
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

var typeMapping = map[string]string{
	"Array":     "List",
	"array":     "List",
	"List":      "List",
	"boolean":   "bool",
	"string":    "String",
	"char":      "String",
	"int":       "int",
	"long":      "int",
	"short":     "int",
	"number":    "num",
	"float":     "double",
	"double":    "double",
	"object":    "Object",
	"integer":   "int",
	"Date":      "DateTime",
	"date":      "DateTime",
	"File":      "MultipartFile",
	"UUID":      "String",
	"binary":    "String",
	"ByteArray": "String",
}

var languageSpecificPrimitives = map[string]bool{
	"String": true,
	"bool":   true,
	"int":    true,
	"num":    true,
	"double": true,
}

// GetSwaggerType maps a Schema to its Dart type string.
func GetSwaggerType(schema *swagger.Schema, definitions map[string]*swagger.Schema) string {
	if schema == nil {
		return "Object"
	}

	// Handle $ref
	if schema.Ref != "" {
		name := swagger.ResolveRef(schema.Ref)
		return ToModelName(name)
	}

	// Handle array
	if schema.Type == "array" {
		inner := GetTypeDeclaration(schema.Items, definitions)
		return "List<" + inner + ">"
	}

	// Handle map (object with additionalProperties)
	if schema.Type == "object" && schema.AdditionalProperties != nil {
		inner := GetTypeDeclaration(schema.AdditionalProperties, definitions)
		return "Map<String, " + inner + ">"
	}

	// Map swagger type + format to Dart type
	swaggerType := schema.Type
	switch schema.Format {
	case "date-time":
		swaggerType = "Date"
	case "float":
		swaggerType = "float"
	case "double":
		swaggerType = "double"
	case "int64", "int32":
		swaggerType = "integer"
	}

	if mapped, ok := typeMapping[swaggerType]; ok {
		if languageSpecificPrimitives[mapped] {
			return mapped
		}
		return ToModelName(mapped)
	}

	if swaggerType != "" {
		return ToModelName(swaggerType)
	}

	return "Object"
}

// GetTypeDeclaration returns the full Dart type declaration for a schema.
func GetTypeDeclaration(schema *swagger.Schema, definitions map[string]*swagger.Schema) string {
	if schema == nil {
		return "Object"
	}

	if schema.Ref != "" {
		name := swagger.ResolveRef(schema.Ref)
		return ToModelName(name)
	}

	if schema.Type == "array" {
		inner := GetTypeDeclaration(schema.Items, definitions)
		return "List<" + inner + ">"
	}

	if schema.Type == "object" && schema.AdditionalProperties != nil {
		inner := GetTypeDeclaration(schema.AdditionalProperties, definitions)
		return "Map<String, " + inner + ">"
	}

	return GetSwaggerType(schema, definitions)
}

// IsPrimitiveType checks if a Dart type is a language primitive.
func IsPrimitiveType(dartType string) bool {
	return languageSpecificPrimitives[dartType]
}
