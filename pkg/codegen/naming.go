package codegen

import (
	"regexp"
	"strings"
	"unicode"
)

var dartReservedWords = map[string]bool{
	"abstract": true, "as": true, "assert": true, "async": true, "async*": true,
	"await": true, "break": true, "case": true, "catch": true, "class": true,
	"const": true, "continue": true, "default": true, "deferred": true, "do": true,
	"dynamic": true, "else": true, "enum": true, "export": true, "external": true,
	"extends": true, "factory": true, "false": true, "final": true, "finally": true,
	"for": true, "get": true, "if": true, "implements": true, "import": true,
	"in": true, "is": true, "library": true, "new": true, "null": true,
	"operator": true, "part": true, "rethrow": true, "return": true, "set": true,
	"static": true, "super": true, "switch": true, "sync*": true, "this": true,
	"throw": true, "true": true, "try": true, "typedef": true, "var": true,
	"void": true, "while": true, "with": true, "yield": true, "yield*": true,
}

func IsReservedWord(name string) bool {
	return dartReservedWords[strings.ToLower(name)]
}

func EscapeReservedWord(name string) string {
	return name + "_"
}

// Camelize converts a name to camelCase or PascalCase.
// If lowercaseFirst is true, the first character is lowercased.
func Camelize(name string, lowercaseFirst bool) string {
	if name == "" {
		return name
	}

	// Split on underscores, hyphens, dots, and spaces
	parts := regexp.MustCompile(`[_\-.\s]+`).Split(name, -1)
	var result strings.Builder

	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 && lowercaseFirst {
			runes := []rune(part)
			runes[0] = unicode.ToLower(runes[0])
			result.WriteString(string(runes))
		} else {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			result.WriteString(string(runes))
		}
	}

	return result.String()
}

// Underscore converts a CamelCase name to snake_case.
func Underscore(name string) string {
	if name == "" {
		return name
	}

	var result strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
					result.WriteRune('_')
				}
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ToVarName converts a property name to a Dart variable name.
func ToVarName(name string) string {
	// Replace hyphens with underscores
	name = strings.ReplaceAll(name, "-", "_")

	// If it's all uppercase, keep it
	if regexp.MustCompile(`^[A-Z_]*$`).MatchString(name) {
		return name
	}

	// Camelize with lowercase first character
	name = Camelize(name, true)

	// Prefix with 'n' if starts with digit
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "n" + name
	}

	if IsReservedWord(name) {
		name = EscapeReservedWord(name)
	}

	return name
}

// ToParamName converts a parameter name to a Dart parameter name.
func ToParamName(name string) string {
	return ToVarName(name)
}

// ToModelName converts a definition name to a Dart class name.
func ToModelName(name string) string {
	if IsReservedWord(name) {
		name = "model_" + name
	}
	return Camelize(name, false)
}

// ToModelFilename converts a model name to a Dart filename (snake_case).
func ToModelFilename(name string) string {
	return Underscore(ToModelName(name))
}

// ToApiName converts a tag name to a Dart API class name.
func ToApiName(name string) string {
	return Camelize(name, false) + "Api"
}

// ToApiFilename converts an API name to a Dart filename (snake_case).
func ToApiFilename(name string) string {
	return Underscore(ToApiName(name))
}

// ToOperationId converts an operationId to a Dart method name.
func ToOperationId(operationId string) string {
	if IsReservedWord(operationId) {
		return Camelize("call_"+operationId, true)
	}
	return Camelize(operationId, true)
}

// ToEnumVarName converts an enum value to a Dart enum variable name.
func ToEnumVarName(value, datatype string) string {
	if value == "" {
		return "empty"
	}
	v := regexp.MustCompile(`\W+`).ReplaceAllString(value, "_")
	if strings.EqualFold(datatype, "number") || strings.EqualFold(datatype, "int") {
		v = "Number" + v
	}
	return EscapeReservedWord(Camelize(v, true))
}

// ToEnumValue converts an enum value to its Dart representation.
func ToEnumValue(value, datatype string) string {
	if strings.EqualFold(datatype, "number") || strings.EqualFold(datatype, "int") {
		return value
	}
	return `"` + EscapeText(value) + `"`
}

// EscapeText escapes special characters in text for Dart strings.
func EscapeText(input string) string {
	input = strings.ReplaceAll(input, "\\", "\\\\")
	input = strings.ReplaceAll(input, "\"", "\\\"")
	input = strings.ReplaceAll(input, "\n", "\\n")
	input = strings.ReplaceAll(input, "\r", "\\r")
	input = strings.ReplaceAll(input, "\t", "\\t")
	return input
}

// FindCommonPrefix finds the common prefix of enum values for truncation.
func FindCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, v := range values[1:] {
		for !strings.HasPrefix(v, prefix) {
			prefix = prefix[:len(prefix)-1]
			if prefix == "" {
				return ""
			}
		}
	}
	return prefix
}
