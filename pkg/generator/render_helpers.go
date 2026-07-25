package generator

import (
	"fmt"
	"html"
	"strings"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
)

// escapeText mimics Java DefaultCodegen.escapeText():
// collapse whitespace, escape backslash, escape double-quotes.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// htmlEscape applies Mustache-style HTML entity escaping ({{var}} behavior).
func htmlEscape(s string) string {
	return html.EscapeString(s)
}

// renderVarDeclarations renders the property declarations section of a class.
// Matches the exact Mustache output format.
//
// It takes the whole model rather than just the vars because a class that
// extends BaseModel redeclares BaseModel's `id`, and Dart's annotate_overrides
// lint wants an @override on it. Without one every such model has to be
// hand-edited after generation.
func renderVarDeclarations(model codegen.ModelData) string {
	var sb strings.Builder
	for _, v := range model.Vars {
		if v.Description != "" {
			sb.WriteString("/* ")
			sb.WriteString(escapeText(v.Description))
			sb.WriteString(" */\n")
		} else {
			sb.WriteString("\n")
		}
		if model.HasId && v.Name == "id" {
			sb.WriteString("      @override\n")
		}
		if v.IsListContainer {
			sb.WriteString("      ")
			sb.WriteString(v.Datatype)
			sb.WriteString("? ")
			sb.WriteString(v.Name)
			sb.WriteString(" = ")
			sb.WriteString(v.DefaultValue)
			sb.WriteString(";\n")
		} else {
			sb.WriteString("      ")
			sb.WriteString(v.Datatype)
			sb.WriteString("? ")
			sb.WriteString(v.Name)
			sb.WriteString(";\n")
		}
		sb.WriteString("  \n")
		// Output inline enum comment if property has enum values
		if len(v.EnumValues) > 0 {
			sb.WriteString("  //enum ")
			sb.WriteString(v.Name)
			sb.WriteString("Enum {")
			for _, ev := range v.EnumValues {
				sb.WriteString("  ")
				sb.WriteString(ev)
				sb.WriteString(",")
			}
			sb.WriteString("  };\n")
		}
	}
	// Add trailing "  " to match Mustache's "  $" line between vars section and next section
	sb.WriteString("  ")
	return sb.String()
}

// renderConstructorParams renders the constructor parameter list (this.name,this.name,...,).
func renderConstructorParams(vars []codegen.PropertyData) string {
	var sb strings.Builder
	for _, v := range vars {
		sb.WriteString("this.")
		sb.WriteString(v.Name)
		sb.WriteString(",")
	}
	return sb.String()
}

// renderToString renders the toString body variables.
func renderToStringVars(vars []codegen.PropertyData) string {
	var sb strings.Builder
	for _, v := range vars {
		sb.WriteString(v.Name)
		sb.WriteString("=$")
		sb.WriteString(v.Name)
		sb.WriteString(", ")
	}
	return sb.String()
}

// renderFromJsonBody renders the fromJson assignment body.
// Matches exact Mustache output whitespace patterns.
func renderFromJsonBody(vars []codegen.PropertyData) string {
	var sb strings.Builder
	for _, v := range vars {
		if v.IsDateTime {
			// DateTime: single line
			fmt.Fprintf(&sb, "    %s = json['%s'] == null ? null : DateTime.parse(json['%s']);\n",
				v.Name, v.BaseName, v.BaseName)
		} else if v.IsNativeEnumRef {
			// A Dart enum has no constructors to call; the generated free
			// functions do the decoding, and unknown wire values land on the
			// enum's unknown member instead of throwing.
			fmt.Fprintf(&sb, "    %s = json['%s'] == null ? null : %sFromJson(json['%s']);\n",
				v.Name, v.BaseName, codegen.Camelize(v.ComplexType, true), v.BaseName)
		} else if v.IsMapContainer {
			// Maps are handled ahead of the ComplexType branch because a map's
			// value can be a list or a primitive, neither of which ComplexType
			// describes. Assigning json['x'] straight to a typed map — which is
			// what the primitive case used to do — throws at runtime.
			renderMapFromJson(&sb, v)
		} else if v.ComplexType != "" {
			// Complex type
			fmt.Fprintf(&sb, "    %s =\n", v.Name)
			if v.IsListContainer {
				// Complex list: 6 spaces indent, ; at column 0
				fmt.Fprintf(&sb, "      json['%s'] == null ? [] : %s.listFromJson(json['%s'])\n;\n",
					v.BaseName, v.ComplexType, v.BaseName)
			} else {
				// Complex non-list, non-map: 2 blank lines from Mustache nesting, ; at column 0
				fmt.Fprintf(&sb, "      \n      \n      json['%s'] == null ? null : %s.fromJson(json['%s'])\n;\n",
					v.BaseName, v.ComplexType, v.BaseName)
			}
		} else {
			// No complex type
			fmt.Fprintf(&sb, "    %s =\n", v.Name)
			if v.IsListContainer {
				// Simple list: 8 spaces indent, ; at 4 spaces (no extra blank lines)
				fmt.Fprintf(&sb, "        json['%s'] == null ? null : (json['%s'] as List).map((item) => item as %s).toList()\n    ;\n",
					v.BaseName, v.BaseName, v.Items.Datatype)
			} else if v.IsDouble {
				// Double: 8 spaces indent, ; at 4 spaces
				fmt.Fprintf(&sb, "        json['%s'] == null ? null : json['%s'].toDouble()\n    ;\n",
					v.BaseName, v.BaseName)
			} else {
				// Simple scalar: 8 spaces indent, ; at 4 spaces
				fmt.Fprintf(&sb, "        json['%s']\n    ;\n",
					v.BaseName)
			}
		}
	}
	return sb.String()
}

// renderMapFromJson renders the fromJson assignment for a map-valued property.
//
// json.decode hands back Map<String, dynamic>, so every shape has to be
// rebuilt into the declared map type rather than assigned across. The four
// shapes are: a map of models, a map of lists (of models or of primitives), a
// map of doubles, and a map of any other primitive.
func renderMapFromJson(sb *strings.Builder, v codegen.PropertyData) {
	source := fmt.Sprintf("json['%s']", v.BaseName)
	fmt.Fprintf(sb, "    %s = %s == null\n        ? {}\n        : ", v.Name, source)

	switch {
	case v.MapValueIsList:
		element := v.MapValueComplexType
		if element != "" {
			// Map<String, List<Model>>
			fmt.Fprintf(sb, "(%s as Map<String, dynamic>).map((key, value) =>\n            MapEntry(key, %s.listFromJson(value as List<dynamic>)))",
				source, element)
		} else {
			// Map<String, List<primitive>>
			fmt.Fprintf(sb, "(%s as Map<String, dynamic>).map((key, value) =>\n            MapEntry(key, (value as List).map((item) => item as %s).toList()))",
				source, mapListElementType(v.MapValueDatatype))
		}
	case v.MapValueComplexType != "":
		// Map<String, Model>
		fmt.Fprintf(sb, "%s.mapFromJson(%s as Map<String, dynamic>)", v.MapValueComplexType, source)
	case v.MapValueIsDouble:
		// JSON numbers decode as int when they have no fraction, so a plain
		// cast to double fails on values like 3.
		fmt.Fprintf(sb, "(%s as Map<String, dynamic>).map(\n            (key, value) => MapEntry(key, (value as num).toDouble()))", source)
	default:
		fmt.Fprintf(sb, "Map<String, %s>.from(%s as Map)", v.MapValueDatatype, source)
	}

	sb.WriteString(";\n")
}

// elementFromJson renders the expression that turns one decoded JSON element
// into the Dart element type.
//
// Numbers need the num detour: JSON gives back int for a whole number even
// where the schema says double, so a straight `as double` throws.
func elementFromJson(datatype string, expr string) string {
	switch datatype {
	case "String":
		return expr + ".toString()"
	case "int":
		return "(" + expr + " as num).toInt()"
	case "double":
		return "(" + expr + " as num).toDouble()"
	case "bool", "Object", "dynamic":
		return expr + " as " + datatype
	default:
		if IsDartPrimitive(datatype) {
			return expr + " as " + datatype
		}
		// A generated model.
		return datatype + ".fromJson(" + expr + ")"
	}
}

// IsDartPrimitive reports whether a rendered Dart type needs no fromJson.
func IsDartPrimitive(datatype string) bool {
	switch datatype {
	case "String", "int", "double", "num", "bool", "Object", "dynamic":
		return true
	}
	return strings.HasPrefix(datatype, "List<") || strings.HasPrefix(datatype, "Map<")
}

// mapListElementType extracts T from "List<T>", for a map whose values are
// lists of primitives.
func mapListElementType(datatype string) string {
	inner := strings.TrimPrefix(datatype, "List<")
	inner = strings.TrimSuffix(inner, ">")
	if inner == "" || inner == datatype {
		return "dynamic"
	}
	return inner
}

// renderToJsonBody renders the toJson entries.
func renderToJsonBody(vars []codegen.PropertyData) string {
	var sb strings.Builder
	for i, v := range vars {
		isLast := i == len(vars)-1
		comma := ""
		if !isLast {
			comma = ","
		}
		if v.IsDateTime {
			fmt.Fprintf(&sb, "      '%s': %s == null ? '' : %s!.toUtc().toIso8601String()%s\n",
				v.BaseName, v.Name, v.Name, comma)
		} else if v.IsNativeEnumRef {
			// A Dart enum is not JSON-encodable; the free function maps it back
			// to its wire value.
			fmt.Fprintf(&sb, "      '%s': %sToJson(%s)%s\n",
				v.BaseName, codegen.Camelize(v.ComplexType, true), v.Name, comma)
		} else {
			fmt.Fprintf(&sb, "      '%s': %s%s\n",
				v.BaseName, v.Name, comma)
		}
	}
	return sb.String()
}

// renderCopyWithParams renders the copyWith parameter declarations.
func renderCopyWithParams(vars []codegen.PropertyData) string {
	var sb strings.Builder
	for _, v := range vars {
		fmt.Fprintf(&sb, "    %s? %s,\n", v.Datatype, v.Name)
	}
	return sb.String()
}

// renderCopyWithArgs renders the copyWith constructor arguments.
func renderCopyWithArgs(vars []codegen.PropertyData) string {
	var sb strings.Builder
	for _, v := range vars {
		fmt.Fprintf(&sb, "        %s: %s ?? this.%s,\n", v.Name, v.Name, v.Name)
	}
	return sb.String()
}

// renderOperations renders all operations for an API class.
func renderOperations(ops []codegen.OperationData) string {
	var sb strings.Builder
	for _, op := range ops {
		renderOperation(&sb, &op)
	}
	return sb.String()
}

func renderOperation(sb *strings.Builder, op *codegen.OperationData) {
	// Summary comment
	fmt.Fprintf(sb, "  /// %s\n", htmlEscape(op.Summary))
	sb.WriteString("  ///\n")
	fmt.Fprintf(sb, "  /// %s\n", htmlEscape(op.Notes))

	// Deprecated annotation
	if op.IsDeprecated {
		sb.WriteString("  @deprecated\n")
	}

	// Method signature
	if op.ReturnType != "" {
		fmt.Fprintf(sb, "  Future<%s?> ", op.ReturnType)
	} else {
		sb.WriteString("  Future ")
	}
	fmt.Fprintf(sb, "%s(", op.Nickname)

	// Required params first
	first := true
	for _, p := range op.AllParams {
		if p.Required {
			if !first {
				sb.WriteString(", ")
			}
			fmt.Fprintf(sb, "%s %s", p.DataType, p.ParamName)
			first = false
		}
	}

	// Optional params in named block
	if op.HasOptionalParams {
		if !first {
			sb.WriteString(", ")
		}
		sb.WriteString("{ ")
		firstOpt := true
		for _, p := range op.AllParams {
			if !p.Required {
				if !firstOpt {
					sb.WriteString(", ")
				}
				fmt.Fprintf(sb, "%s? %s", p.DataType, p.ParamName)
				firstOpt = false
			}
		}
		sb.WriteString(", String? contentType }")
	} else if op.HasParams {
		sb.WriteString(", { String? contentType }")
	} else {
		sb.WriteString("{ String? contentType }")
	}

	sb.WriteString(") async {\n")

	// Post body
	if op.BodyParam != nil {
		fmt.Fprintf(sb, "    Object? postBody = %s;\n", op.BodyParam.ParamName)
	} else {
		sb.WriteString("    Object? postBody;\n")
	}
	sb.WriteString("\n")

	// Path
	sb.WriteString("    // create path and map variables\n")
	pathStr := fmt.Sprintf("    String path = '%s'", op.Path)
	for _, p := range op.PathParams {
		pathStr += fmt.Sprintf(".replaceAll('{' '%s' '}', %s.toString())", p.BaseName, p.ParamName)
	}
	sb.WriteString(pathStr)
	sb.WriteString(";\n")
	sb.WriteString("\n")

	// Query params, header params, form params declarations
	sb.WriteString("    // query params\n")
	sb.WriteString("    List<QueryParam> queryParams = [];\n")
	sb.WriteString("    Map<String, String> headerParams = {};\n")
	sb.WriteString("    Map<String, String> formParams = {};\n")

	// Query param additions
	for _, p := range op.QueryParams {
		if !p.Required {
			fmt.Fprintf(sb, "    if(%s != null) {\n", p.ParamName)
		}
		fmt.Fprintf(sb, "      queryParams.addAll(_convertParametersForCollectionFormat('%s', '%s', %s));\n",
			p.CollectionFormat, p.BaseName, p.ParamName)
		if !p.Required {
			sb.WriteString("    }\n")
		}
	}

	// Header params
	for _, p := range op.HeaderParams {
		fmt.Fprintf(sb, "    headerParams['%s'] = %s;\n", p.BaseName, p.ParamName)
	}

	// Trailing "    " line from Mustache headerParams section
	sb.WriteString("    \n")

	// Content types
	sb.WriteString("    List<String> contentTypes = [")
	for i, c := range op.Consumes {
		fmt.Fprintf(sb, "'%s'", c.MediaType)
		if i < len(op.Consumes)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("];\n")
	sb.WriteString("\n")
	sb.WriteString("    contentType ??= contentTypes.first;\n")
	sb.WriteString("\n")

	// Auth names
	sb.WriteString("    List<String> authNames = [")
	for i, a := range op.AuthMethods {
		fmt.Fprintf(sb, "'%s'", a.Name)
		if i < len(op.AuthMethods)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("];\n")
	sb.WriteString("\n")

	// Form params handling
	if op.HasFormParams {
		sb.WriteString("    if(contentType.startsWith('multipart/form-data')) {\n")
		fmt.Fprintf(sb, "      MultipartRequest mp = MultipartRequest('%s', Uri.parse(path));\n", op.HttpMethod)
		for _, p := range op.FormParams {
			if p.NotFile {
				fmt.Fprintf(sb, "      \n")
				if !p.Required {
					fmt.Fprintf(sb, "        mp.fields['%s'] = parameterToString(%s);\n", p.BaseName, p.ParamName)
				} else {
					fmt.Fprintf(sb, "        mp.fields['%s'] = parameterToString(%s);\n", p.BaseName, p.ParamName)
				}
			}
			if p.IsFile {
				sb.WriteString("      \n")
				if !p.Required {
					fmt.Fprintf(sb, "        if(%s != null) {\n", p.ParamName)
				}
				fmt.Fprintf(sb, "        mp.fields['%s'] = %s.field;\n", p.BaseName, p.ParamName)
				fmt.Fprintf(sb, "        mp.files.add(%s);\n", p.ParamName)
				if !p.Required {
					sb.WriteString("        }\n")
				}
			}
		}
		sb.WriteString("      \n")
		sb.WriteString("        postBody = mp;\n")
		sb.WriteString("    }\n")
		sb.WriteString("    else {\n")
		hasNonFileFormParams := false
		firstNonFile := true
		for _, p := range op.FormParams {
			if p.NotFile {
				hasNonFileFormParams = true
				if firstNonFile {
					sb.WriteString("      \n")
					firstNonFile = false
				} else {
					sb.WriteString("\n")
				}
				if !p.Required {
					fmt.Fprintf(sb, "      if (%s != null) {\n", p.ParamName)
				}
				fmt.Fprintf(sb, "        formParams['%s'] = parameterToString(%s);\n", p.BaseName, p.ParamName)
				if !p.Required {
					sb.WriteString("        }\n")
				}
			}
		}
		if hasNonFileFormParams {
			sb.WriteString("    }\n")
		} else {
			// Mustache artifact: extra indentation when no non-file form params
			sb.WriteString("          }\n")
		}
	}
	sb.WriteString("\n")

	// API call
	fmt.Fprintf(sb, "    var response = await apiClient.invokeAPI(path, '%s', queryParams, postBody, headerParams, formParams, contentType, authNames);\n",
		op.HttpMethod)
	sb.WriteString("\n")

	// Response handling
	sb.WriteString("    if(response.statusCode >= 400) {\n")
	sb.WriteString("      throw ApiException(response.statusCode, response.body);\n")
	sb.WriteString("    } else if(response.body.isNotEmpty) {\n")

	// Return statement
	renderReturnStatement(sb, op)

	sb.WriteString("    } else {\n")
	if op.ReturnType != "" {
		sb.WriteString("      return null;\n")
	} else {
		sb.WriteString("      return;\n")
	}
	sb.WriteString("    }\n")
	sb.WriteString("  }\n")
}

// renderReadmeApiTable renders the API endpoints table for README.md.
func renderReadmeApiTable(apis []codegen.ApiData, apiDocPath string) string {
	var sb strings.Builder
	for _, api := range apis {
		for _, op := range api.Operations {
			anchor := strings.ToLower(op.Nickname)
			fmt.Fprintf(&sb, "*%s* | [**%s**](%s/%s.md#%s) | **%s** %s | %s\n",
				api.Classname, op.Nickname, apiDocPath, api.Classname, anchor,
				op.HttpMethod, op.Path, htmlEscape(op.Summary))
		}
	}
	return sb.String()
}

// renderReadmeModelList renders the model documentation list for README.md.
func renderReadmeModelList(models []codegen.ModelData, modelDocPath string) string {
	var sb strings.Builder
	for _, m := range models {
		fmt.Fprintf(&sb, " - [%s](%s/%s.md)\n", m.Classname, modelDocPath, m.Classname)
	}
	return sb.String()
}

// renderReadmeAuth renders the authorization documentation for README.md.
func renderReadmeAuth(authMethods []codegen.AuthMethodData) string {
	var sb strings.Builder
	for _, auth := range authMethods {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "## %s\n", auth.Name)
		sb.WriteString("\n")
		if auth.IsApiKey {
			sb.WriteString("- **Type**: API key\n")
			fmt.Fprintf(&sb, "- **API key parameter name**: %s\n", auth.KeyParamName)
			if auth.IsKeyInHeader {
				sb.WriteString("- **Location**: HTTP header\n")
			} else {
				sb.WriteString("- **Location**: URL query string\n")
			}
		} else if auth.IsBasic {
			sb.WriteString("- **Type**: HTTP basic authentication\n")
		} else if auth.IsOAuth {
			sb.WriteString("- **Type**: OAuth\n")
			sb.WriteString("- **Flow**: implicit\n")
		}
	}
	return sb.String()
}

func renderReturnStatement(sb *strings.Builder, op *codegen.OperationData) {
	if op.IsListContainer && op.ReturnType != "" {
		// List return: multiline
		sb.WriteString("      return \n")
		fmt.Fprintf(sb, "        (apiClient.deserialize(utf8.decode(response.bodyBytes), '%s') as List).map((item) => item as %s).toList();\n",
			op.ReturnType, op.ReturnBaseType)
	} else if op.IsMapContainer && op.ReturnType != "" {
		// Map return
		fmt.Fprintf(sb, "      return           %s.from(apiClient.deserialize(utf8.decode(response.bodyBytes), '%s')) ;\n",
			op.ReturnType, op.ReturnType)
	} else if op.ReturnType != "" {
		// Simple return with value
		fmt.Fprintf(sb, "      return           apiClient.deserialize(utf8.decode(response.bodyBytes), '%s') as %s ;\n",
			op.ReturnType, op.ReturnType)
	} else {
		// Void return
		sb.WriteString("      return           ;\n")
	}
}
