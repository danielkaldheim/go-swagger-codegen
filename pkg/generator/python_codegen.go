package generator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

// PythonGenerationData is the complete template context for a generated
// Python package. Python has deliberately separate template data from Dart:
// sharing already-rendered Dart type names made a second language look easy,
// but produced subtly invalid clients as soon as containers or reserved words
// appeared in a specification.
type PythonGenerationData struct {
	PackageName        string
	DistributionName   string
	PackageVersion     string
	PackageDescription string
	PythonRequires     string
	BasePath           string
	AppVersion         string
	AppDescription     string
	InfoEmail          string
	Models             []PythonModel
	Apis               []PythonAPI
	AuthMethods        []PythonAuthMethod
}

type PythonModel struct {
	ClassName   string
	Description string
	IsEnum      bool
	IsAlias     bool
	AliasType   string
	Fields      []PythonField
	EnumBase    string
	EnumMembers []PythonEnumMember
}

type PythonField struct {
	Name        string
	WireName    string
	DataType    string
	Required    bool
	Description string
}

type PythonEnumMember struct {
	Name      string
	Value     string
	IsUnknown bool
}

type PythonAPI struct {
	ClassName   string
	Description string
	Operations  []PythonOperation
}

type PythonOperation struct {
	MethodName  string
	HTTPMethod  string
	Path        string
	Summary     string
	Description string
	ReturnType  string
	HasReturn   bool
	Deprecated  bool
	Parameters  []PythonParameter
	AuthNames   []string
	ContentType string
	HasFiles    bool
}

type PythonParameter struct {
	Name             string
	WireName         string
	DataType         string
	Required         bool
	Location         string
	CollectionFormat string
	IsFile           bool
}

type PythonAuthMethod struct {
	Name     string
	Type     string
	Location string
	KeyName  string
}

func buildPythonData(spec *swagger.Spec, cfg pythonConfig, models []codegen.ModelData, apis []codegen.ApiData) PythonGenerationData {
	allowedModels := make(map[string]bool, len(models))
	for _, model := range models {
		allowedModels[model.Classname] = true
	}
	allowedAPIs := make(map[string]bool, len(apis))
	for _, api := range apis {
		allowedAPIs[api.Classname] = true
	}

	distributionName := cfg.DistributionName
	if distributionName == "" {
		distributionName = strings.ReplaceAll(cfg.PackageName, "_", "-")
	}

	return PythonGenerationData{
		PackageName:        cfg.PackageName,
		DistributionName:   distributionName,
		PackageVersion:     cfg.PackageVersion,
		PackageDescription: cfg.PackageDescription,
		PythonRequires:     cfg.PythonRequires,
		BasePath:           pythonBasePath(spec),
		AppVersion:         spec.Info.Version,
		AppDescription:     spec.Info.Description,
		InfoEmail:          spec.Info.Contact.Email,
		Models:             buildPythonModels(spec, cfg.NativeEnums, cfg.UseEnumExtension, allowedModels),
		Apis:               buildPythonAPIs(spec, allowedAPIs),
		AuthMethods:        buildPythonAuthMethods(spec.SecurityDefinitions),
	}
}

type pythonConfig struct {
	PackageName        string
	DistributionName   string
	PackageVersion     string
	PackageDescription string
	PythonRequires     string
	NativeEnums        map[string][]string
	UseEnumExtension   bool
}

func pythonBasePath(spec *swagger.Spec) string {
	scheme := "https"
	if len(spec.Schemes) > 0 && spec.Schemes[0] != "" {
		scheme = spec.Schemes[0]
	}
	if spec.Host == "" {
		return strings.TrimRight(spec.BasePath, "/")
	}
	return strings.TrimRight(scheme+"://"+spec.Host+spec.BasePath, "/")
}

func buildPythonModels(spec *swagger.Spec, nativeEnums map[string][]string, useEnumExtension bool, allowed map[string]bool) []PythonModel {
	names := append([]string(nil), spec.DefinitionOrder...)
	if len(names) == 0 {
		for name := range spec.Definitions {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	models := make([]PythonModel, 0, len(names))
	for _, name := range names {
		// Selection is performed by the existing reachability/exclusion pass.
		// It uses Dart class names today, so compare with the exact key it emits.
		if !allowed[codegen.ToModelName(name)] {
			continue
		}
		schema := spec.Definitions[name]
		if schema == nil {
			continue
		}
		model := PythonModel{
			ClassName:   pythonClassName(name),
			Description: pythonCollapseNewlines(schema.Description),
		}

		enumValues, enumNames := pythonEnumValues(schema, useEnumExtension)
		if len(enumValues) == 0 {
			if configured, ok := nativeEnums[name]; ok {
				for _, value := range configured {
					enumValues = append(enumValues, value)
					enumNames = append(enumNames, "")
				}
			}
		}
		if len(enumValues) > 0 {
			model.IsEnum = true
			model.EnumBase = pythonEnumBase(enumValues)
			model.EnumMembers = buildPythonEnumMembers(enumValues, enumNames, model.EnumBase)
			models = append(models, model)
			continue
		}

		if len(schema.Properties) == 0 && (schema.Ref != "" || (schema.Type != "object" && schema.Type != "")) {
			model.IsAlias = true
			model.AliasType = pythonTypeDeclaration(schema)
			models = append(models, model)
			continue
		}
		if len(schema.Properties) == 0 && schema.Type == "object" && schema.AdditionalProperties != nil {
			model.IsAlias = true
			model.AliasType = pythonTypeDeclaration(schema)
			models = append(models, model)
			continue
		}

		required := make(map[string]bool, len(schema.Required))
		for _, field := range schema.Required {
			required[field] = true
		}
		fieldNames := make([]string, 0, len(schema.Properties))
		for field := range schema.Properties {
			fieldNames = append(fieldNames, field)
		}
		// Dataclasses require fields without defaults to precede fields with
		// defaults. Sort within those groups for deterministic output.
		sort.Slice(fieldNames, func(i, j int) bool {
			if required[fieldNames[i]] != required[fieldNames[j]] {
				return required[fieldNames[i]]
			}
			return fieldNames[i] < fieldNames[j]
		})
		usedFieldNames := map[string]bool{"from_dict": true, "to_dict": true}
		for _, fieldName := range fieldNames {
			fieldSchema := schema.Properties[fieldName]
			fieldType := pythonTypeDeclaration(fieldSchema)
			isRequired := required[fieldName]
			if !isRequired {
				fieldType += " | None"
			}
			description := ""
			if fieldSchema != nil {
				description = pythonCollapseNewlines(fieldSchema.Description)
			}
			model.Fields = append(model.Fields, PythonField{
				Name:        uniquePythonName(pythonIdentifier(fieldName), usedFieldNames),
				WireName:    fieldName,
				DataType:    fieldType,
				Required:    isRequired,
				Description: description,
			})
		}
		models = append(models, model)
	}
	return models
}

func pythonEnumValues(schema *swagger.Schema, useEnumExtension bool) ([]any, []string) {
	if len(schema.Enum) > 0 {
		return append([]any(nil), schema.Enum...), make([]string, len(schema.Enum))
	}
	if !useEnumExtension {
		return nil, nil
	}
	extension, ok := schema.VendorExtensions["x-enum-values"].([]any)
	if !ok {
		return nil, nil
	}
	values := make([]any, 0, len(extension))
	names := make([]string, 0, len(extension))
	for _, raw := range extension {
		value, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		numericValue, exists := value["numericValue"]
		if !exists {
			continue
		}
		identifier, _ := value["identifier"].(string)
		values = append(values, numericValue)
		names = append(names, identifier)
	}
	return values, names
}

func pythonEnumBase(values []any) string {
	allStrings, allInts, allNumbers := true, true, true
	for _, value := range values {
		switch number := value.(type) {
		case string:
			allInts, allNumbers = false, false
		case float64:
			allStrings = false
			if number != float64(int64(number)) {
				allInts = false
			}
		case int, int32, int64:
			allStrings = false
		default:
			allStrings, allInts, allNumbers = false, false, false
		}
	}
	switch {
	case allStrings:
		return "str"
	case allInts:
		return "int"
	case allNumbers:
		return "float"
	default:
		return ""
	}
}

func buildPythonEnumMembers(values []any, suggestedNames []string, enumBase string) []PythonEnumMember {
	used := make(map[string]int)
	members := make([]PythonEnumMember, 0, len(values)+1)
	for index, value := range values {
		name := ""
		if index < len(suggestedNames) {
			name = suggestedNames[index]
		}
		if name == "" {
			name = fmt.Sprint(value)
		}
		name = pythonEnumName(name)
		if name == "UNKNOWN" {
			name = "VALUE_UNKNOWN"
		}
		if count := used[name]; count > 0 {
			used[name] = count + 1
			name += "_" + strconv.Itoa(count+1)
		} else {
			used[name] = 1
		}
		members = append(members, PythonEnumMember{Name: name, Value: pythonLiteral(value)})
	}
	// String enums use a safe unknown fallback, which is particularly useful
	// for long-lived Home Assistant installations talking to a newer server.
	if enumBase == "str" {
		wireValues := make(map[string]bool, len(values))
		for _, value := range values {
			wireValues[fmt.Sprint(value)] = true
		}
		unknown := ""
		for wireValues[unknown] {
			unknown = "_" + unknown + "unknown_"
		}
		members = append(members, PythonEnumMember{Name: "UNKNOWN", Value: pythonLiteral(unknown), IsUnknown: true})
	}
	return members
}

func buildPythonAPIs(spec *swagger.Spec, allowed map[string]bool) []PythonAPI {
	type taggedOperation struct {
		path   string
		method string
		op     *swagger.Operation
	}
	byTag := make(map[string][]taggedOperation)
	paths := append([]string(nil), spec.PathOrder...)
	if len(paths) == 0 {
		for path := range spec.Paths {
			paths = append(paths, path)
		}
		sort.Strings(paths)
	}
	for _, path := range paths {
		item := spec.Paths[path]
		methods := []struct {
			name string
			op   *swagger.Operation
		}{
			{"GET", item.Get}, {"POST", item.Post}, {"PUT", item.Put},
			{"DELETE", item.Delete}, {"PATCH", item.Patch},
		}
		for _, method := range methods {
			if method.op == nil {
				continue
			}
			tag := "Default"
			if len(method.op.Tags) > 0 && method.op.Tags[0] != "" {
				tag = method.op.Tags[0]
			}
			if !allowed[codegen.ToApiName(tag)] {
				continue
			}
			byTag[tag] = append(byTag[tag], taggedOperation{path: path, method: method.name, op: method.op})
		}
	}

	tags := make([]string, 0, len(byTag))
	for tag := range byTag {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	apis := make([]PythonAPI, 0, len(tags))
	for _, tag := range tags {
		api := PythonAPI{ClassName: pythonClassName(tag) + "Api", Description: tag + " endpoints."}
		usedMethodNames := map[string]bool{"__init__": true}
		for _, item := range byTag[tag] {
			operation := buildPythonOperation(item.path, item.method, item.op, spec)
			operation.MethodName = uniquePythonName(operation.MethodName, usedMethodNames)
			api.Operations = append(api.Operations, operation)
		}
		sort.SliceStable(api.Operations, func(i, j int) bool {
			return api.Operations[i].MethodName < api.Operations[j].MethodName
		})
		apis = append(apis, api)
	}
	return apis
}

func buildPythonOperation(path, method string, operation *swagger.Operation, spec *swagger.Spec) PythonOperation {
	methodName := operation.OperationID
	if methodName == "" {
		methodName = strings.ToLower(method) + "_" + strings.Trim(path, "/")
	}
	result := PythonOperation{
		MethodName:  pythonIdentifier(methodName),
		HTTPMethod:  method,
		Path:        path,
		Summary:     pythonCollapseNewlines(operation.Summary),
		Description: pythonCollapseNewlines(operation.Description),
		Deprecated:  operation.Deprecated,
		ContentType: "application/json",
	}

	consumes := operation.Consumes
	if len(consumes) == 0 {
		consumes = spec.Consumes
	}
	if len(consumes) > 0 {
		result.ContentType = consumes[0]
	}

	usedParameterNames := map[string]bool{
		"request_path": true, "request_query": true, "request_headers": true,
		"request_body": true, "request_form": true, "request_files": true,
	}
	for _, param := range operation.Parameters {
		pythonParam := PythonParameter{
			Name:             uniquePythonName(pythonIdentifier(param.Name), usedParameterNames),
			WireName:         param.Name,
			Required:         param.Required,
			Location:         param.In,
			CollectionFormat: param.CollectionFormat,
		}
		switch {
		case param.In == "body" && param.Schema != nil:
			pythonParam.DataType = pythonTypeDeclaration(param.Schema)
		case param.Type == "file":
			pythonParam.DataType = "FileValue"
			pythonParam.IsFile = true
			result.HasFiles = true
		case param.Type == "array":
			pythonParam.DataType = "list[" + pythonTypeDeclaration(param.Items) + "]"
		default:
			pythonParam.DataType = pythonTypeDeclaration(&swagger.Schema{Type: param.Type, Format: param.Format, Items: param.Items})
		}
		if !param.Required {
			pythonParam.DataType += " | None"
		}
		result.Parameters = append(result.Parameters, pythonParam)
	}
	// Keep generated Python signatures legal and deterministic.
	sort.SliceStable(result.Parameters, func(i, j int) bool {
		if result.Parameters[i].Required != result.Parameters[j].Required {
			return result.Parameters[i].Required
		}
		return false
	})
	if result.HasFiles {
		result.ContentType = "multipart/form-data"
	}

	for _, code := range []string{"200", "201", "202", "203", "204"} {
		response, ok := operation.Responses[code]
		if !ok || response.Schema == nil {
			continue
		}
		result.ReturnType = pythonTypeDeclaration(response.Schema)
		result.HasReturn = true
		break
	}
	if !result.HasReturn {
		result.ReturnType = "None"
	}

	security := spec.Security
	if operation.SecurityPresent {
		security = operation.Security
	}
	if len(security) > 0 {
		for name := range security[0] {
			result.AuthNames = append(result.AuthNames, name)
		}
		sort.Strings(result.AuthNames)
	}
	return result
}

func buildPythonAuthMethods(definitions map[string]swagger.SecurityDefinition) []PythonAuthMethod {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	methods := make([]PythonAuthMethod, 0, len(names))
	for _, name := range names {
		definition := definitions[name]
		methods = append(methods, PythonAuthMethod{
			Name: name, Type: definition.Type, Location: definition.In, KeyName: definition.Name,
		})
	}
	return methods
}

func pythonTypeDeclaration(schema *swagger.Schema) string {
	if schema == nil {
		return "Any"
	}
	if schema.Ref != "" {
		return pythonClassName(swagger.ResolveRef(schema.Ref))
	}
	if schema.Type == "array" {
		return "list[" + pythonTypeDeclaration(schema.Items) + "]"
	}
	if schema.Type == "object" && schema.AdditionalProperties != nil {
		return "dict[str, " + pythonTypeDeclaration(schema.AdditionalProperties) + "]"
	}
	switch schema.Format {
	case "date-time":
		return "datetime"
	case "date":
		return "date"
	case "uuid":
		return "UUID"
	case "byte", "binary":
		return "bytes"
	}
	switch schema.Type {
	case "string":
		return "str"
	case "integer":
		return "int"
	case "number":
		return "float"
	case "boolean":
		return "bool"
	case "object", "":
		return "Any"
	case "file":
		// Swagger's file type is handled as FileValue for form parameters.
		// Inside a model it is data, not a transport object.
		return "bytes"
	default:
		return pythonClassName(schema.Type)
	}
}

var pythonSeparators = regexp.MustCompile(`[^[:alnum:]_]+`)
var pythonLeadingDigits = regexp.MustCompile(`^[0-9]+`)

func pythonClassName(name string) string {
	name = pythonSeparators.ReplaceAllString(name, "_")
	name = codegen.Camelize(name, false)
	if name == "" {
		return "Model"
	}
	if pythonLeadingDigits.MatchString(name) {
		name = "Model" + name
	}
	if pythonClassReserved[name] {
		name = "Model" + name
	}
	return name
}

var pythonClassReserved = map[string]bool{
	"Any": true, "ApiClient": true, "ApiError": true, "Enum": true,
	"FileValue": true, "Mapping": true, "Self": true, "TypeAlias": true,
	"UUID": true, "Union": true,
}

func pythonIdentifier(name string) string {
	name = pythonSeparators.ReplaceAllString(name, "_")
	name = codegen.Underscore(codegen.Camelize(name, false))
	name = strings.ToLower(strings.Trim(name, "_"))
	if name == "" {
		name = "value"
	}
	if unicode.IsDigit([]rune(name)[0]) {
		name = "value_" + name
	}
	if pythonReservedWords[name] || pythonAnnotationNames[name] || name == "self" || name == "cls" {
		name += "_"
	}
	return name
}

// pythonAnnotationNames are subscripted built-in types emitted directly in
// generated annotations. A dataclass field lives in the class namespace, so a
// field named `list` makes a later annotation such as `list[Recipe]` resolve
// to that field instead of the built-in type. Escape only names the generator
// subscripts, rather than every Python built-in, so API fields such as `id`,
// `date`, and `bytes` retain their natural spelling.
var pythonAnnotationNames = map[string]bool{
	"dict": true, "list": true,
}

var pythonReservedWords = map[string]bool{
	"and": true, "as": true, "assert": true, "async": true, "await": true,
	"break": true, "case": true, "class": true, "continue": true, "def": true,
	"del": true, "elif": true, "else": true, "except": true, "false": true,
	"finally": true, "for": true, "from": true, "global": true, "if": true,
	"import": true, "in": true, "is": true, "lambda": true, "match": true,
	"none": true, "nonlocal": true, "not": true, "or": true, "pass": true,
	"raise": true, "return": true, "true": true, "try": true, "while": true,
	"with": true, "yield": true,
}

func pythonPackageName(name string) string {
	if strings.Trim(pythonSeparators.ReplaceAllString(name, ""), "_") == "" {
		return "swagger_client"
	}
	return pythonIdentifier(name)
}

func pythonEnumName(value string) string {
	name := pythonSeparators.ReplaceAllString(value, "_")
	name = codegen.Underscore(codegen.Camelize(name, false))
	name = strings.ToUpper(strings.Trim(name, "_"))
	if name == "" {
		name = "EMPTY"
	}
	if unicode.IsDigit([]rune(name)[0]) {
		name = "VALUE_" + name
	}
	return name
}

func uniquePythonName(name string, used map[string]bool) string {
	if !used[name] {
		used[name] = true
		return name
	}
	for suffix := 2; ; suffix++ {
		candidate := name + "_" + strconv.Itoa(suffix)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

func pythonLiteral(value any) string {
	if value == nil {
		return "None"
	}
	switch typed := value.(type) {
	case string:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'g', -1, 64)
	default:
		return fmt.Sprint(value)
	}
}

func pythonCollapseNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.ReplaceAll(value, "\r", " ")
}
