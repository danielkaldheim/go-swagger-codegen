package codegen

import (
	"sort"
	"strings"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

type ApiData struct {
	Classname     string
	ClassFilename string
	ClassVarName  string
	Operations    []OperationData
}

type OperationData struct {
	Nickname        string
	OperationId     string
	HttpMethod      string
	Path            string
	Summary         string
	Notes           string
	ReturnType      string
	ReturnBaseType  string
	IsListContainer bool
	IsMapContainer  bool
	IsDeprecated    bool
	HasOptionalParams bool
	HasParams       bool
	HasFormParams   bool
	HasMore         bool
	AllParams       []ParamData
	PathParams      []ParamData
	QueryParams     []ParamData
	HeaderParams    []ParamData
	BodyParam       *ParamData
	FormParams      []ParamData
	Consumes        []MediaType
	AuthMethods     []AuthMethodRef
}

type ParamData struct {
	ParamName        string
	BaseName         string
	DataType         string
	Required         bool
	IsPathParam      bool
	IsQueryParam     bool
	IsHeaderParam    bool
	IsBodyParam      bool
	IsFormParam      bool
	IsFile           bool
	NotFile          bool
	IsListContainer  bool
	HasMore          bool
	CollectionFormat string
	Description      string
	Example          string
}

type MediaType struct {
	MediaType string
	HasMore   bool
}

type AuthMethodRef struct {
	Name    string
	HasMore bool
}

type AuthMethodData struct {
	Name           string
	IsBasic        bool
	IsApiKey       bool
	IsOAuth        bool
	IsKeyInHeader  bool
	IsKeyInQuery   bool
	KeyParamName   string
}

// BuildApis extracts API data from swagger paths, grouped by tag.
func BuildApis(spec *swagger.Spec) []ApiData {
	apiMap := make(map[string]*ApiData)

	// Use PathOrder to preserve JSON key order from the original spec
	paths := spec.PathOrder
	if len(paths) == 0 {
		// Fallback: sort paths if no order was preserved
		paths = make([]string, 0, len(spec.Paths))
		for p := range spec.Paths {
			paths = append(paths, p)
		}
		sort.Strings(paths)
	}

	for _, path := range paths {
		pathItem := spec.Paths[path]
		methods := map[string]*swagger.Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
			op := methods[method]
			if op == nil {
				continue
			}

			tag := "Default"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}

			api, ok := apiMap[tag]
			if !ok {
				apiName := ToApiName(tag)
				api = &ApiData{
					Classname:     apiName,
					ClassFilename: ToApiFilename(tag),
					ClassVarName:  Camelize(tag, true),
				}
				apiMap[tag] = api
			}

			operation := buildOperation(path, method, op, spec)
			api.Operations = append(api.Operations, operation)
		}
	}

	// Convert map to sorted slice
	tags := make([]string, 0, len(apiMap))
	for tag := range apiMap {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	apis := make([]ApiData, 0, len(tags))
	for _, tag := range tags {
		api := apiMap[tag]
		// Sort operations by nickname (matches Java codegen behavior)
		sort.Slice(api.Operations, func(i, j int) bool {
			return api.Operations[i].Nickname < api.Operations[j].Nickname
		})
		// Set HasMore on operations
		for i := range api.Operations {
			api.Operations[i].HasMore = i < len(api.Operations)-1
		}
		apis = append(apis, *api)
	}

	return apis
}

func buildOperation(path, method string, op *swagger.Operation, spec *swagger.Spec) OperationData {
	nickname := ToOperationId(op.OperationID)

	operation := OperationData{
		Nickname:     nickname,
		OperationId:  nickname,
		HttpMethod:   method,
		Path:         path,
		Summary:      collapseNewlines(op.Summary),
		Notes:        collapseNewlines(op.Description),
		IsDeprecated: op.Deprecated,
	}

	// Consumes
	consumes := op.Consumes
	if len(consumes) == 0 {
		consumes = spec.Consumes
	}
	for i, c := range consumes {
		operation.Consumes = append(operation.Consumes, MediaType{
			MediaType: c,
			HasMore:   i < len(consumes)-1,
		})
	}

	// Auth methods
	var securityReqs []map[string][]string
	if len(op.Security) > 0 {
		securityReqs = op.Security
	}
	for _, sec := range securityReqs {
		for name := range sec {
			operation.AuthMethods = append(operation.AuthMethods, AuthMethodRef{Name: name})
		}
	}
	for i := range operation.AuthMethods {
		operation.AuthMethods[i].HasMore = i < len(operation.AuthMethods)-1
	}

	// Parameters
	for _, param := range op.Parameters {
		pd := buildParam(param, spec.Definitions)

		operation.AllParams = append(operation.AllParams, pd)
		operation.HasParams = true

		switch param.In {
		case "path":
			pd.IsPathParam = true
			operation.PathParams = append(operation.PathParams, pd)
		case "query":
			pd.IsQueryParam = true
			operation.QueryParams = append(operation.QueryParams, pd)
		case "header":
			pd.IsHeaderParam = true
			operation.HeaderParams = append(operation.HeaderParams, pd)
		case "body":
			pd.IsBodyParam = true
			operation.BodyParam = &pd
		case "formData":
			pd.IsFormParam = true
			operation.FormParams = append(operation.FormParams, pd)
			operation.HasFormParams = true
		}

		if !param.Required {
			operation.HasOptionalParams = true
		}
	}

	// Sort AllParams: required first, then optional (matches Java codegen)
	sort.SliceStable(operation.AllParams, func(i, j int) bool {
		if operation.AllParams[i].Required != operation.AllParams[j].Required {
			return operation.AllParams[i].Required
		}
		return false
	})

	// Set HasMore on params
	for i := range operation.AllParams {
		operation.AllParams[i].HasMore = i < len(operation.AllParams)-1
	}
	for i := range operation.QueryParams {
		operation.QueryParams[i].HasMore = i < len(operation.QueryParams)-1
	}

	// Return type from successful response (check 200, 201, 202 etc.)
	var successResp *swagger.Response
	for _, code := range []string{"200", "201", "202", "203", "204"} {
		if resp, ok := op.Responses[code]; ok && resp.Schema != nil {
			successResp = &resp
			break
		}
	}
	if successResp != nil {
		operation.ReturnType = GetTypeDeclaration(successResp.Schema, spec.Definitions)
		if successResp.Schema.Type == "array" {
			operation.IsListContainer = true
			if successResp.Schema.Items != nil && successResp.Schema.Items.Ref != "" {
				operation.ReturnBaseType = ToModelName(swagger.ResolveRef(successResp.Schema.Items.Ref))
			} else if successResp.Schema.Items != nil {
				operation.ReturnBaseType = GetSwaggerType(successResp.Schema.Items, spec.Definitions)
			}
		} else if successResp.Schema.Ref != "" {
			operation.ReturnBaseType = ToModelName(swagger.ResolveRef(successResp.Schema.Ref))
		} else {
			operation.ReturnBaseType = operation.ReturnType
		}
	}

	return operation
}

func buildParam(param swagger.Parameter, definitions map[string]*swagger.Schema) ParamData {
	pd := ParamData{
		ParamName:        ToParamName(param.Name),
		BaseName:         param.Name,
		Required:         param.Required,
		Description:      param.Description,
		CollectionFormat: param.CollectionFormat,
	}

	if param.In == "body" && param.Schema != nil {
		pd.DataType = GetTypeDeclaration(param.Schema, definitions)
	} else if param.Type == "file" {
		pd.DataType = "MultipartFile"
		pd.IsFile = true
		pd.NotFile = false
	} else if param.Type == "array" && param.Items != nil {
		inner := GetTypeDeclaration(param.Items, definitions)
		pd.DataType = "List<" + inner + ">"
		pd.IsListContainer = true
	} else {
		// Build a temporary schema from the parameter
		tempSchema := &swagger.Schema{
			Type:   param.Type,
			Format: param.Format,
		}
		pd.DataType = GetSwaggerType(tempSchema, definitions)
	}

	pd.NotFile = !pd.IsFile

	// Remove "contentType" conflict with the template variable
	if strings.ToLower(pd.ParamName) == "contenttype" {
		// contentType is handled separately in the template
	}

	return pd
}

// BuildAuthMethods extracts authentication method data from security definitions.
func BuildAuthMethods(secDefs map[string]swagger.SecurityDefinition) []AuthMethodData {
	var methods []AuthMethodData
	names := make([]string, 0, len(secDefs))
	for name := range secDefs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		def := secDefs[name]
		m := AuthMethodData{
			Name:         name,
			KeyParamName: def.Name,
		}
		switch def.Type {
		case "basic":
			m.IsBasic = true
		case "apiKey":
			m.IsApiKey = true
			m.IsKeyInHeader = def.In == "header"
			m.IsKeyInQuery = def.In == "query"
		case "oauth2":
			m.IsOAuth = true
		}
		methods = append(methods, m)
	}
	return methods
}

func collapseNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
