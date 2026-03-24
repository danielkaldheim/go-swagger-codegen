package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
)

// mustacheEscape mimics Java Mustache's HTML escaping behavior.
// It uses &quot; for " (not &#34;) and &#39; for ' to match the Java output.
func mustacheEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	s = strings.ReplaceAll(s, "`", "&#x60;")
	s = strings.ReplaceAll(s, "=", "&#x3D;")
	return s
}

func (g *Generator) generateDocs() error {
	docsDir := filepath.Join(g.outputDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("creating docs directory: %w", err)
	}

	gd := g.globalData()

	// Generate model docs
	for _, model := range g.models {
		content := renderModelDoc(model, gd.PubName)
		outputPath := filepath.Join(docsDir, model.Classname+".md")
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing model doc %s: %w", model.Classname, err)
		}
	}

	// Generate API docs
	for _, api := range g.apis {
		content := renderApiDoc(api, gd)
		outputPath := filepath.Join(docsDir, api.Classname+".md")
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing API doc %s: %w", api.Classname, err)
		}
	}

	return nil
}

func renderModelDoc(model codegen.ModelData, pubName string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# %s.model.%s\n", pubName, model.Classname)
	sb.WriteString("\n## Load the model package\n")
	sb.WriteString("```dart\n")
	fmt.Fprintf(&sb, "import 'package:%s/api.dart';\n", pubName)
	sb.WriteString("```\n")
	sb.WriteString("\n## Properties\n")
	sb.WriteString("Name | Type | Description | Notes\n")
	sb.WriteString("------------ | ------------- | ------------- | -------------\n")

	for _, v := range model.Vars {
		typeStr := docPropertyType(v)
		desc := mustacheEscape(escapeText(v.Description))

		notes := ""
		if !v.Required {
			notes = "[optional] "
		}
		if v.IsListContainer {
			notes += "[default to []]"
		} else if v.IsMapContainer {
			notes += "[default to {}]"
		} else {
			notes += "[default to null]"
		}

		fmt.Fprintf(&sb, "**%s** | %s | %s | %s\n", v.Name, typeStr, desc, notes)
	}

	sb.WriteString("\n[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)\n")
	sb.WriteString("\n\n")

	return sb.String()
}

func docPropertyType(v codegen.PropertyData) string {
	if v.IsListContainer {
		inner := ""
		if v.Items != nil {
			inner = v.Items.Datatype
		}
		// Primitive inner types: bold only, no link
		if codegen.IsPrimitiveType(inner) {
			return fmt.Sprintf("**List&lt;%s&gt;**", inner)
		}
		// Object links to empty name (Java codegen behavior)
		if inner == "Object" {
			return fmt.Sprintf("[**List&lt;%s&gt;**](.md)", inner)
		}
		// Complex inner types: linked
		return fmt.Sprintf("[**List&lt;%s&gt;**](%s.md)", inner, inner)
	}
	if v.IsMapContainer {
		// HTML-encode the full generic type
		escaped := strings.ReplaceAll(v.Datatype, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		// Determine inner value type for the link target
		linkTarget := v.ComplexType
		if linkTarget == "" && v.Items != nil {
			linkTarget = v.Items.Datatype
		}
		// Check if the map contains Object values (detect from Datatype)
		isObjectMap := linkTarget == "" && strings.Contains(v.Datatype, "Object")
		// If the inner value type is primitive and not Object, use bold only (no link)
		if linkTarget == "" && !isObjectMap {
			return fmt.Sprintf("**%s**", escaped)
		}
		if codegen.IsPrimitiveType(linkTarget) {
			return fmt.Sprintf("**%s**", escaped)
		}
		// Object maps link to empty name (Java codegen behavior)
		if linkTarget == "Object" || isObjectMap {
			return fmt.Sprintf("[**%s**](.md)", escaped)
		}
		return fmt.Sprintf("[**%s**](%s.md)", escaped, linkTarget)
	}
	if v.ComplexType != "" {
		return fmt.Sprintf("[**%s**](%s.md)", v.Datatype, v.ComplexType)
	}
	if v.IsDateTime {
		return "[**DateTime**](DateTime.md)"
	}
	if codegen.IsPrimitiveType(v.Datatype) {
		return fmt.Sprintf("**%s**", v.Datatype)
	}
	// Object links to empty name
	if v.Datatype == "Object" {
		return "[**Object**](.md)"
	}
	// Default: link to the type
	return fmt.Sprintf("[**%s**](%s.md)", v.Datatype, v.Datatype)
}

func renderApiDoc(api codegen.ApiData, gd GlobalData) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# %s.api.%s\n", gd.PubName, api.Classname)
	sb.WriteString("\n## Load the API package\n")
	sb.WriteString("```dart\n")
	fmt.Fprintf(&sb, "import 'package:%s/api.dart';\n", gd.PubName)
	sb.WriteString("```\n")
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "All URIs are relative to *%s*\n", gd.BasePath)
	sb.WriteString("\nMethod | HTTP request | Description\n")
	sb.WriteString("------------- | ------------- | -------------\n")

	// Method summary table
	for _, op := range api.Operations {
		fmt.Fprintf(&sb, "[**%s**](%s.md#%s) | **%s** %s | %s\n",
			op.Nickname, api.Classname, op.Nickname, op.HttpMethod, op.Path, mustacheEscape(op.Summary))
	}

	sb.WriteString("\n")

	// Per-operation details
	for _, op := range api.Operations {
		renderApiOpDoc(&sb, &op, &api, &gd)
	}

	sb.WriteString("\n")

	return sb.String()
}

func renderApiOpDoc(sb *strings.Builder, op *codegen.OperationData, api *codegen.ApiData, gd *GlobalData) {
	sb.WriteString("\n")

	// Header
	fmt.Fprintf(sb, "# **%s**\n", op.Nickname)

	// Signature line
	sb.WriteString("> ")
	if op.ReturnType != "" {
		fmt.Fprintf(sb, "%s ", op.ReturnType)
	}
	fmt.Fprintf(sb, "%s(", op.Nickname)
	for i, p := range op.AllParams {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.ParamName)
	}
	sb.WriteString(")\n")

	// Summary and notes
	sb.WriteString("\n")
	fmt.Fprintf(sb, "%s\n", mustacheEscape(op.Summary))
	sb.WriteString("\n")
	// Notes may already contain markdown formatting (e.g., # headings)
	fmt.Fprintf(sb, "%s\n", op.Notes)

	// Example
	sb.WriteString("\n### Example \n")
	sb.WriteString("```dart\n")
	fmt.Fprintf(sb, "import 'package:%s/api.dart';\n", gd.PubName)

	// Auth config
	for _, auth := range op.AuthMethods {
		for _, am := range gd.AuthMethods {
			if am.Name == auth.Name {
				if am.IsApiKey {
					fmt.Fprintf(sb, "// TODO Configure API key authorization: %s\n", am.Name)
					fmt.Fprintf(sb, "//%s.api.Configuration.apiKey{'%s'} = 'YOUR_API_KEY';\n", gd.PubName, am.KeyParamName)
					sb.WriteString("// uncomment below to setup prefix (e.g. Bearer) for API key, if needed\n")
					fmt.Fprintf(sb, "//%s.api.Configuration.apiKeyPrefix{'%s'} = \"Bearer\";\n", gd.PubName, am.KeyParamName)
				} else if am.IsOAuth {
					fmt.Fprintf(sb, "// TODO Configure OAuth2 access token for authorization: %s\n", am.Name)
					fmt.Fprintf(sb, "//%s.api.Configuration.accessToken = 'YOUR_ACCESS_TOKEN';\n", gd.PubName)
				}
				break
			}
		}
	}

	sb.WriteString("\n")
	fmt.Fprintf(sb, "var api_instance = %s();\n", api.Classname)

	// Param var declarations (descriptions go through escapeText like Java codegen)
	for _, p := range op.AllParams {
		desc := escapeText(p.Description)
		if p.IsBodyParam {
			fmt.Fprintf(sb, "var %s = %s(); // %s | %s\n", p.ParamName, p.DataType, p.DataType, desc)
		} else {
			example := docParamExample(p)
			fmt.Fprintf(sb, "var %s = %s; // %s | %s\n", p.ParamName, example, p.DataType, desc)
		}
	}

	// Try/catch
	sb.WriteString("\ntry { \n")
	if op.ReturnType != "" {
		sb.WriteString("    var result = ")
	} else {
		sb.WriteString("    ")
	}
	fmt.Fprintf(sb, "api_instance.%s(", op.Nickname)
	for i, p := range op.AllParams {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(p.ParamName)
	}
	sb.WriteString(");\n")
	if op.ReturnType != "" {
		sb.WriteString("    print(result);\n")
	}
	sb.WriteString("} catch (e) {\n")
	fmt.Fprintf(sb, "    print(\"Exception when calling %s->%s: $e\\n\");\n", api.Classname, op.Nickname)
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	// Parameters section
	sb.WriteString("\n### Parameters\n")
	if len(op.AllParams) > 0 {
		sb.WriteString("\nName | Type | Description  | Notes\n")
		sb.WriteString("------------- | ------------- | ------------- | -------------\n")
		for _, p := range op.AllParams {
			typeStr := docParamType(p)
			notes := ""
			if !p.Required {
				notes = "[optional] "
			}
			if p.DefaultValue != "" {
				notes += fmt.Sprintf("[default to %s]", p.DefaultValue)
			}
			desc := mustacheEscape(escapeText(p.Description))
			fmt.Fprintf(sb, " **%s** | %s| %s | %s\n", p.ParamName, typeStr, desc, notes)
		}
	} else {
		sb.WriteString("This endpoint does not need any parameter.\n")
	}

	// Return type
	sb.WriteString("\n### Return type\n\n")
	if op.ReturnType != "" {
		if codegen.IsPrimitiveType(op.ReturnType) {
			fmt.Fprintf(sb, "**%s**\n", op.ReturnType)
		} else {
			// For List/Map return types, link to the base type, not the full generic
			linkTarget := op.ReturnType
			if op.ReturnBaseType != "" {
				linkTarget = op.ReturnBaseType
			}
			fmt.Fprintf(sb, "[**%s**](%s.md)\n", op.ReturnType, linkTarget)
		}
	} else {
		sb.WriteString("void (empty response body)\n")
	}

	// Authorization
	sb.WriteString("\n### Authorization\n\n")
	if len(op.AuthMethods) > 0 {
		for _, auth := range op.AuthMethods {
			fmt.Fprintf(sb, "[%s](../README.md#%s)\n", auth.Name, auth.Name)
		}
	} else {
		sb.WriteString("No authorization required\n")
	}

	// HTTP request headers
	sb.WriteString("\n### HTTP request headers\n\n")
	if len(op.Consumes) > 0 {
		sb.WriteString(" - **Content-Type**: ")
		for i, c := range op.Consumes {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(c.MediaType)
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString(" - **Content-Type**: Not defined\n")
	}
	sb.WriteString(" - **Accept**: application/json\n")

	sb.WriteString("\n[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)\n")
}

func docParamType(p codegen.ParamData) string {
	if p.IsBodyParam {
		if codegen.IsPrimitiveType(p.DataType) {
			return fmt.Sprintf("**%s**", p.DataType)
		}
		return fmt.Sprintf("[**%s**](%s.md)", p.DataType, p.DataType)
	}
	if p.IsFile {
		return "**MultipartFile**"
	}
	if codegen.IsPrimitiveType(p.DataType) {
		return fmt.Sprintf("**%s**", p.DataType)
	}
	return fmt.Sprintf("[**%s**](%s.md)", p.DataType, p.DataType)
}

func docParamExample(p codegen.ParamData) string {
	if p.IsFile {
		return "/path/to/file.txt"
	}
	switch p.DataType {
	case "String":
		return p.ParamName + "_example"
	case "int":
		return "789"
	case "bool":
		return "true"
	case "double", "num":
		return "1.2"
	default:
		return p.ParamName + "_example"
	}
}
