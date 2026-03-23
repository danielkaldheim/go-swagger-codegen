package codegen

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

type ModelData struct {
	Classname        string
	ClassFilename    string
	Description      string
	IsEnum           bool
	IsAlias          bool
	HasVars          bool
	HasId            bool
	DataType         string // for enums/aliases, the underlying type
	Vars             []PropertyData
	AllowableValues  *AllowableValues
	VendorExtensions map[string]any
}

type PropertyData struct {
	Name            string
	BaseName        string
	Datatype        string
	ComplexType     string
	IsListContainer bool
	IsMapContainer  bool
	IsDateTime      bool
	IsDouble        bool
	IsPrimitiveType bool
	Required        bool
	ReadOnly        bool
	Description     string
	DefaultValue    string
	Items           *ItemData
	IsLast          bool
	EnumValues      []string // inline enum values for property-level enums
}

type ItemData struct {
	Datatype string
}

type AllowableValues struct {
	Values   []any
	EnumVars []EnumVar
}

type EnumVar struct {
	Name        string
	Value       string
	Description string
}

// BuildModels extracts model data from swagger definitions.
func BuildModels(definitions map[string]*swagger.Schema, defs map[string]*swagger.Schema, useEnumExtension bool, definitionOrder []string) []ModelData {
	var models []ModelData

	// Use definitionOrder to preserve JSON key order, fallback to sorted
	names := definitionOrder
	if len(names) == 0 {
		names = make([]string, 0, len(definitions))
		for name := range definitions {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	for _, name := range names {
		schema := definitions[name]
		model := buildModel(name, schema, defs, useEnumExtension)
		models = append(models, model)
	}

	return models
}

func buildModel(name string, schema *swagger.Schema, definitions map[string]*swagger.Schema, useEnumExtension bool) ModelData {
	classname := ToModelName(name)
	model := ModelData{
		Classname:        classname,
		ClassFilename:    ToModelFilename(name),
		Description:      schema.Description,
		VendorExtensions: schema.VendorExtensions,
	}

	// Determine the underlying data type for the schema
	model.DataType = GetSwaggerType(schema, definitions)

	// Check if it's an enum
	if len(schema.Enum) > 0 {
		model.IsEnum = true
		model.AllowableValues = &AllowableValues{}
		for _, v := range schema.Enum {
			model.AllowableValues.Values = append(model.AllowableValues.Values, v)
		}
	}

	// Check x-enum-values vendor extension
	if useEnumExtension && !model.IsEnum {
		if extValues, ok := schema.VendorExtensions["x-enum-values"]; ok {
			model.IsEnum = true
			model.IsAlias = false
			model.AllowableValues = &AllowableValues{}
			if vals, ok := extValues.([]any); ok {
				for _, v := range vals {
					if m, ok := v.(map[string]any); ok {
						if nv, ok := m["numericValue"]; ok {
							model.AllowableValues.Values = append(model.AllowableValues.Values, nv)
						}
					}
				}
			}
		}
	}

	// If it has properties, build them
	if len(schema.Properties) > 0 {
		model.IsEnum = false
		model.IsAlias = false
		model.HasVars = true

		requiredSet := make(map[string]bool)
		for _, r := range schema.Required {
			requiredSet[r] = true
		}

		// Sort property names for deterministic output
		propNames := make([]string, 0, len(schema.Properties))
		for pn := range schema.Properties {
			propNames = append(propNames, pn)
		}
		sort.Strings(propNames)

		for _, propName := range propNames {
			prop := schema.Properties[propName]
			pd := buildProperty(propName, prop, definitions, requiredSet[propName])

			if propName == "id" {
				model.HasId = true
			}

			// Clear complexType for Object types
			if pd.ComplexType == "Object" {
				pd.ComplexType = ""
			}

			model.Vars = append(model.Vars, pd)
		}

		// Mark last var
		if len(model.Vars) > 0 {
			model.Vars[len(model.Vars)-1].IsLast = true
		}
	} else if !model.IsEnum {
		// Determine if this is a type alias or an empty class.
		// In Java codegen, object types and array types become classes,
		// while simple scalar types (string, integer) become type aliases.
		switch {
		case schema.Type == "object" || schema.Type == "" || schema.Type == "array":
			// Empty class - not an alias
			model.IsAlias = false
			model.DataType = "Object"
		default:
			// Simple scalar type alias (e.g., string, integer)
			model.IsAlias = true
		}
	}

	// Build enum vars
	if model.IsEnum && model.AllowableValues != nil {
		buildEnumVars(&model, useEnumExtension)
	}

	return model
}

func buildProperty(name string, schema *swagger.Schema, definitions map[string]*swagger.Schema, required bool) PropertyData {
	pd := PropertyData{
		Name:     ToVarName(name),
		BaseName: name,
		Required: required,
	}

	if schema == nil {
		pd.Datatype = "Object"
		return pd
	}

	// Collapse multi-line descriptions to single line (matches Java codegen behavior)
	desc := schema.Description
	desc = strings.ReplaceAll(desc, "\r\n", " ")
	desc = strings.ReplaceAll(desc, "\n", " ")
	pd.Description = desc

	// Handle $ref
	if schema.Ref != "" {
		refName := swagger.ResolveRef(schema.Ref)
		pd.Datatype = ToModelName(refName)
		pd.ComplexType = ToModelName(refName)
		return pd
	}

	// Handle array
	if schema.Type == "array" {
		pd.IsListContainer = true
		inner := GetTypeDeclaration(schema.Items, definitions)
		pd.Datatype = "List<" + inner + ">"
		pd.DefaultValue = "[]"

		// Set items datatype
		pd.Items = &ItemData{Datatype: inner}

		// Set complex type if inner is not primitive
		if schema.Items != nil && schema.Items.Ref != "" {
			refName := swagger.ResolveRef(schema.Items.Ref)
			pd.ComplexType = ToModelName(refName)
		}
		return pd
	}

	// Handle map (object with additionalProperties)
	if schema.Type == "object" && schema.AdditionalProperties != nil {
		pd.IsMapContainer = true
		inner := GetTypeDeclaration(schema.AdditionalProperties, definitions)
		pd.Datatype = "Map<String, " + inner + ">"
		pd.DefaultValue = "{}"

		if schema.AdditionalProperties.Ref != "" {
			refName := swagger.ResolveRef(schema.AdditionalProperties.Ref)
			pd.ComplexType = ToModelName(refName)
		} else if schema.AdditionalProperties.Type != "" {
			// For non-ref additionalProperties, set complexType to the base
			// mapped type (not the full generic). Only set for non-primitives.
			baseType := getBaseSwaggerType(schema.AdditionalProperties.Type)
			if !IsPrimitiveType(baseType) {
				pd.ComplexType = baseType
			}
		}
		return pd
	}

	// Handle date-time
	if schema.Format == "date-time" {
		pd.Datatype = "DateTime"
		pd.IsDateTime = true
		return pd
	}

	// Handle double (Java codegen only sets isDouble for format "double", not "float")
	if schema.Format == "double" {
		pd.Datatype = "double"
		pd.IsDouble = true
		return pd
	}

	// Default type mapping
	pd.Datatype = GetSwaggerType(schema, definitions)
	pd.IsPrimitiveType = IsPrimitiveType(pd.Datatype)

	// Detect inline enum values on the property
	if len(schema.Enum) > 0 {
		for _, v := range schema.Enum {
			pd.EnumValues = append(pd.EnumValues, fmt.Sprintf("%v", v))
		}
	}

	return pd
}

func buildEnumVars(model *ModelData, useEnumExtension bool) {
	// Try vendor extension first
	if useEnumExtension && model.VendorExtensions != nil {
		if extValues, ok := model.VendorExtensions["x-enum-values"]; ok {
			if vals, ok := extValues.([]any); ok {
				for _, v := range vals {
					if m, ok := v.(map[string]any); ok {
						identifier, _ := m["identifier"].(string)
						name := Camelize(identifier, true)
						if IsReservedWord(name) {
							name = EscapeReservedWord(name)
						}
						numericValue := ""
						if nv, ok := m["numericValue"]; ok {
							numericValue = toString(nv)
						}
						ev := EnumVar{
							Name:  name,
							Value: ToEnumValue(numericValue, model.DataType),
						}
						if desc, ok := m["description"]; ok {
							ev.Description = toString(desc)
						}
						model.AllowableValues.EnumVars = append(model.AllowableValues.EnumVars, ev)
					}
				}
				return
			}
		}
	}

	// Build from values
	values := model.AllowableValues.Values
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = toString(v)
	}

	commonPrefix := FindCommonPrefix(strValues)
	truncateIdx := len(commonPrefix)

	for _, value := range values {
		strVal := toString(value)
		var enumName string
		if truncateIdx == 0 {
			enumName = strVal
		} else {
			enumName = strVal[truncateIdx:]
			if enumName == "" {
				enumName = strVal
			}
		}
		model.AllowableValues.EnumVars = append(model.AllowableValues.EnumVars, EnumVar{
			Name:  ToEnumVarName(enumName, model.DataType),
			Value: ToEnumValue(strVal, model.DataType),
		})
	}
}

// getBaseSwaggerType maps a swagger type to its Dart base type (without generics).
func getBaseSwaggerType(swaggerType string) string {
	if mapped, ok := typeMapping[swaggerType]; ok {
		return mapped
	}
	return ToModelName(swaggerType)
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}
