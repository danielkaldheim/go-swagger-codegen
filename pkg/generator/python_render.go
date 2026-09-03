package generator

import (
	"fmt"
	"strings"
)

func renderPythonModels(models []PythonModel) string {
	var out strings.Builder
	// Aliases are runtime assignments on Python 3.11. Emit them after classes
	// and enums so aliases such as list[SomeModel] cannot reference a name that
	// has not been defined yet.
	ordered := make([]PythonModel, 0, len(models))
	for _, model := range models {
		if !model.IsAlias {
			ordered = append(ordered, model)
		}
	}
	for _, model := range models {
		if model.IsAlias {
			ordered = append(ordered, model)
		}
	}
	for _, model := range ordered {
		switch {
		case model.IsEnum:
			base := "Enum"
			if model.EnumBase != "" {
				base = model.EnumBase + ", Enum"
			}
			fmt.Fprintf(&out, "class %s(%s):\n", model.ClassName, base)
			writePythonDoc(&out, model.Description, 1)
			for _, member := range model.EnumMembers {
				fmt.Fprintf(&out, "    %s = %s\n", member.Name, member.Value)
			}
			if len(model.EnumMembers) == 0 {
				out.WriteString("    pass\n")
			}
			hasUnknown := false
			for _, member := range model.EnumMembers {
				hasUnknown = hasUnknown || member.IsUnknown
			}
			if hasUnknown {
				out.WriteString("\n    @classmethod\n")
				out.WriteString("    def _missing_(cls, value: object) -> Self:\n")
				out.WriteString("        return cls.UNKNOWN\n")
			}
		case model.IsAlias:
			if model.Description != "" {
				writePythonComment(&out, model.Description, 0)
			}
			fmt.Fprintf(&out, "%s: TypeAlias = %s\n", model.ClassName, model.AliasType)
		default:
			out.WriteString("@dataclass(slots=True)\n")
			fmt.Fprintf(&out, "class %s:\n", model.ClassName)
			writePythonDoc(&out, model.Description, 1)
			if len(model.Fields) == 0 {
				out.WriteString("    pass\n")
			} else {
				for _, field := range model.Fields {
					if field.Description != "" {
						writePythonComment(&out, field.Description, 1)
					}
					fmt.Fprintf(&out, "    %s: %s", field.Name, field.DataType)
					if !field.Required {
						out.WriteString(" = None")
					}
					out.WriteByte('\n')
				}
			}
			out.WriteString("\n    @classmethod\n")
			out.WriteString("    def from_dict(cls, data: Mapping[str, Any]) -> Self:\n")
			out.WriteString("        if not isinstance(data, Mapping):\n")
			fmt.Fprintf(&out, "            raise TypeError(%s)\n", pythonLiteral(model.ClassName+" must be decoded from an object"))
			if len(model.Fields) == 0 {
				out.WriteString("        return cls()\n")
			} else {
				out.WriteString("        return cls(\n")
				for _, field := range model.Fields {
					accessor := fmt.Sprintf("data[%s]", pythonLiteral(field.WireName))
					if !field.Required {
						accessor = fmt.Sprintf("data.get(%s)", pythonLiteral(field.WireName))
					}
					fmt.Fprintf(&out, "            %s=_deserialize(%s, %s),\n", field.Name, accessor, field.DataType)
				}
				out.WriteString("        )\n")
			}
			out.WriteString("\n    def to_dict(self) -> dict[str, Any]:\n")
			out.WriteString("        result: dict[str, Any] = {}\n")
			for _, field := range model.Fields {
				if field.Required {
					fmt.Fprintf(&out, "        result[%s] = _serialize(self.%s)\n", pythonLiteral(field.WireName), field.Name)
				} else {
					fmt.Fprintf(&out, "        if self.%s is not None:\n", field.Name)
					fmt.Fprintf(&out, "            result[%s] = _serialize(self.%s)\n", pythonLiteral(field.WireName), field.Name)
				}
			}
			out.WriteString("        return result\n")
		}
		out.WriteString("\n\n")
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

func renderPythonAPIs(apis []PythonAPI) string {
	var out strings.Builder
	for _, api := range apis {
		fmt.Fprintf(&out, "class %s:\n", api.ClassName)
		writePythonDoc(&out, api.Description, 1)
		out.WriteString("    def __init__(self, api_client: ApiClient) -> None:\n")
		out.WriteString("        self.api_client = api_client\n")
		for _, operation := range api.Operations {
			out.WriteString("\n    async def ")
			out.WriteString(operation.MethodName)
			out.WriteString("(self")
			hasOptional := false
			for _, parameter := range operation.Parameters {
				if !parameter.Required && !hasOptional {
					out.WriteString(", *")
					hasOptional = true
				}
				fmt.Fprintf(&out, ", %s: %s", parameter.Name, parameter.DataType)
				if !parameter.Required {
					out.WriteString(" = None")
				}
			}
			fmt.Fprintf(&out, ") -> %s:\n", operation.ReturnType)
			doc := operation.Summary
			if operation.Description != "" {
				if doc != "" {
					doc += " "
				}
				doc += operation.Description
			}
			if operation.Deprecated {
				if doc != "" {
					doc += " "
				}
				doc += "Deprecated."
			}
			writePythonDoc(&out, doc, 2)
			fmt.Fprintf(&out, "        request_path = %s\n", pythonLiteral(operation.Path))
			for _, parameter := range operation.Parameters {
				if parameter.Location == "path" {
					fmt.Fprintf(&out, "        request_path = request_path.replace(%s, quote(_query_value(%s), safe=\"\"))\n", pythonLiteral("{"+parameter.WireName+"}"), parameter.Name)
				}
			}
			out.WriteString("        request_query: list[tuple[str, str]] = []\n")
			for _, parameter := range operation.Parameters {
				if parameter.Location == "query" {
					fmt.Fprintf(&out, "        _add_query(request_query, %s, %s, %s)\n", pythonLiteral(parameter.WireName), parameter.Name, pythonLiteral(parameter.CollectionFormat))
				}
			}
			out.WriteString("        request_headers: dict[str, str] = {}\n")
			for _, parameter := range operation.Parameters {
				if parameter.Location != "header" {
					continue
				}
				if !parameter.Required {
					fmt.Fprintf(&out, "        if %s is not None:\n", parameter.Name)
					fmt.Fprintf(&out, "            request_headers[%s] = _query_value(%s)\n", pythonLiteral(parameter.WireName), parameter.Name)
				} else {
					fmt.Fprintf(&out, "        request_headers[%s] = _query_value(%s)\n", pythonLiteral(parameter.WireName), parameter.Name)
				}
			}
			out.WriteString("        request_body: Any = None\n")
			out.WriteString("        request_form: dict[str, Any] = {}\n")
			out.WriteString("        request_files: dict[str, FileValue] = {}\n")
			for _, parameter := range operation.Parameters {
				switch parameter.Location {
				case "body":
					fmt.Fprintf(&out, "        request_body = %s\n", parameter.Name)
				case "formData":
					target := "request_form"
					if parameter.IsFile {
						target = "request_files"
					}
					if !parameter.Required {
						fmt.Fprintf(&out, "        if %s is not None:\n", parameter.Name)
						fmt.Fprintf(&out, "            %s[%s] = %s\n", target, pythonLiteral(parameter.WireName), parameter.Name)
					} else {
						fmt.Fprintf(&out, "        %s[%s] = %s\n", target, pythonLiteral(parameter.WireName), parameter.Name)
					}
				}
			}
			out.WriteString("        return await self.api_client.request(\n")
			fmt.Fprintf(&out, "            %s,\n", pythonLiteral(operation.HTTPMethod))
			out.WriteString("            request_path,\n")
			out.WriteString("            query=request_query,\n")
			out.WriteString("            headers=request_headers,\n")
			out.WriteString("            body=request_body,\n")
			out.WriteString("            form=request_form,\n")
			out.WriteString("            files=request_files,\n")
			fmt.Fprintf(&out, "            content_type=%s,\n", pythonLiteral(operation.ContentType))
			fmt.Fprintf(&out, "            auth_names=%s,\n", pythonStringList(operation.AuthNames))
			if operation.HasReturn {
				fmt.Fprintf(&out, "            response_type=%s,\n", operation.ReturnType)
			} else {
				out.WriteString("            response_type=None,\n")
			}
			out.WriteString("        )\n")
		}
		out.WriteString("\n\n")
	}
	return strings.TrimRight(out.String(), "\n") + "\n"
}

func pythonStringList(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = pythonLiteral(value)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func writePythonDoc(out *strings.Builder, value string, indent int) {
	if value == "" {
		return
	}
	out.WriteString(strings.Repeat("    ", indent))
	out.WriteString(pythonLiteral(value))
	out.WriteByte('\n')
}

func writePythonComment(out *strings.Builder, value string, indent int) {
	prefix := strings.Repeat("    ", indent) + "# "
	for _, line := range strings.Split(value, "\n") {
		out.WriteString(prefix)
		out.WriteString(line)
		out.WriteByte('\n')
	}
}
