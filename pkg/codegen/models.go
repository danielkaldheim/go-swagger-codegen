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
	IsListAlias      bool // top-level `type: array`; DataType is the element type
	IsMapAlias       bool // top-level free-form map; DataType is the value type
	IsNativeEnum     bool // render as a real Dart enum rather than a string wrapper
	EnumMembers      []EnumMember
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
	IsNativeEnumRef bool     // property's type is a generated native Dart enum

	// Map value shape. Set only when IsMapContainer is true, and needed
	// because a map's value can itself be a list or a model — which
	// ComplexType alone cannot express.
	MapValueDatatype    string // "double", "String", "RecipeNutrientTotal", "List<Foo>"
	MapValueComplexType string // model name of the value, or of the list element
	MapValueIsList      bool
	MapValueIsDouble    bool
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

// EnumMember is one member of a native Dart enum. The unknown fallback has an
// empty WireValue: it is what unrecognised input decodes to, and it must never
// be sent back.
type EnumMember struct {
	Name      string
	WireValue string
}

// IsUnknown reports whether this is the fallback member.
func (m EnumMember) IsUnknown() bool { return m.WireValue == "" }

// BuildModels extracts model data from swagger definitions.
func BuildModels(definitions map[string]*swagger.Schema, defs map[string]*swagger.Schema, useEnumExtension bool, definitionOrder []string, nativeEnums map[string][]string) []ModelData {
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
		model := buildModel(name, schema, defs, useEnumExtension, nativeEnums)
		models = append(models, model)
	}

	return models
}

func buildModel(name string, schema *swagger.Schema, definitions map[string]*swagger.Schema, useEnumExtension bool, nativeEnums map[string][]string) ModelData {
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
		model.AllowableValues.Values = append(model.AllowableValues.Values, schema.Enum...)
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
			pd := buildProperty(propName, prop, definitions, requiredSet[propName], nativeEnums)

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
		case schema.Type == "array":
			// A top-level `type: array` definition, e.g. StringArray. Emitting
			// an empty class here silently dropped the whole payload: the class
			// had no field to put the list in, so every value round-tripped to
			// nothing. It becomes a list wrapper instead.
			model.IsListAlias = true
			model.DataType = GetTypeDeclaration(schema.Items, definitions)
		case schema.Type == "object" && schema.AdditionalProperties != nil:
			// A free-form map definition, e.g. JSONMap. Same problem as above.
			model.IsMapAlias = true
			value := schema.AdditionalProperties
			if value.Type == "" && value.Ref == "" {
				// `additionalProperties: {}` means "any JSON". dynamic rather
				// than Object, because Object cannot hold the nulls that
				// arbitrary JSON contains.
				model.DataType = "dynamic"
			} else {
				model.DataType = GetTypeDeclaration(value, definitions)
			}
		case schema.Type == "object" || schema.Type == "":
			// Genuinely empty class - not an alias
			model.IsAlias = false
			model.DataType = "Object"
		default:
			// Simple scalar type alias (e.g., string, integer)
			model.IsAlias = true
		}
	}

	// Native Dart enum. Preference order is deliberate: a definition that
	// carries its own enum list wins, and the config map is only a stand-in for
	// specs that cannot express one.
	if values, ok := nativeEnums[name]; ok {
		model.IsNativeEnum = true
		model.IsAlias = false
		model.IsEnum = false
		model.IsListAlias = false
		model.IsMapAlias = false
		model.EnumMembers = buildNativeEnumMembers(specEnumValues(schema), values)
	}

	// Build enum vars
	if model.IsEnum && model.AllowableValues != nil {
		buildEnumVars(&model, useEnumExtension)
	}

	return model
}

// specEnumValues returns the definition's own enum list as strings, or nil.
func specEnumValues(schema *swagger.Schema) []string {
	if len(schema.Enum) == 0 {
		return nil
	}
	values := make([]string, 0, len(schema.Enum))
	for _, raw := range schema.Enum {
		if s, ok := raw.(string); ok {
			values = append(values, s)
		}
	}
	return values
}

// buildNativeEnumMembers turns wire values into Dart enum members, preferring
// the spec's list over the configured one.
//
// An `unknown` member is always prepended: a client that predates a new server
// value must decode it to something rather than throw, and unknown is the only
// honest answer.
func buildNativeEnumMembers(fromSpec []string, fromConfig []string) []EnumMember {
	values := fromSpec
	if len(values) == 0 {
		values = fromConfig
	}

	members := make([]EnumMember, 0, len(values)+1)
	members = append(members, EnumMember{Name: NativeEnumUnknownMember})
	for _, value := range values {
		if value == "" || value == NativeEnumUnknownMember {
			continue
		}
		members = append(members, EnumMember{
			Name:      Camelize(strings.ReplaceAll(value, "-", "_"), true),
			WireValue: value,
		})
	}
	return members
}

// NativeEnumUnknownMember is the fallback member every generated Dart enum
// carries, for wire values this client has never heard of.
const NativeEnumUnknownMember = "unknown"

func buildProperty(name string, schema *swagger.Schema, definitions map[string]*swagger.Schema, required bool, nativeEnums map[string][]string) PropertyData {
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
		if _, ok := nativeEnums[refName]; ok {
			// A native Dart enum has no fromJson/toJson of its own; it is read
			// and written through free functions instead.
			pd.IsNativeEnumRef = true
		}
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
		value := schema.AdditionalProperties
		inner := GetTypeDeclaration(value, definitions)
		pd.Datatype = "Map<String, " + inner + ">"
		pd.DefaultValue = "{}"
		pd.MapValueDatatype = inner
		pd.MapValueIsDouble = inner == "double"

		switch {
		case value.Ref != "":
			refName := ToModelName(swagger.ResolveRef(value.Ref))
			pd.ComplexType = refName
			pd.MapValueComplexType = refName
		case value.Type == "array":
			// A map whose values are lists. ComplexType is deliberately left
			// empty here: it used to be set to the base type of "array", which
			// is the literal "List", and the renderer then emitted
			// `List.mapFromJson(...)` — valid Dart that always throws.
			pd.MapValueIsList = true
			if value.Items != nil && value.Items.Ref != "" {
				pd.MapValueComplexType = ToModelName(swagger.ResolveRef(value.Items.Ref))
			}
		case value.Type != "":
			baseType := getBaseSwaggerType(value.Type)
			if !IsPrimitiveType(baseType) {
				pd.ComplexType = baseType
				pd.MapValueComplexType = baseType
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
