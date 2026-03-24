package swagger

import (
	"bytes"
	"encoding/json"
)

type Spec struct {
	Swagger             string                        `json:"swagger"`
	Info                Info                          `json:"info"`
	Host                string                        `json:"host"`
	BasePath            string                        `json:"basePath"`
	Schemes             []string                      `json:"schemes"`
	Consumes            []string                      `json:"consumes"`
	Produces            []string                      `json:"produces"`
	Paths               map[string]PathItem           `json:"paths"`
	Definitions         map[string]*Schema            `json:"definitions"`
	SecurityDefinitions map[string]SecurityDefinition `json:"securityDefinitions"`

	// PathOrder preserves the order of paths from the original JSON
	PathOrder []string `json:"-"`
	// DefinitionOrder preserves the order of definitions from the original JSON
	DefinitionOrder []string `json:"-"`
}

type Info struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Version     string  `json:"version"`
	Contact     Contact `json:"contact"`
	License     License `json:"license"`
}

type Contact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
}

type Operation struct {
	Tags        []string              `json:"tags"`
	Summary     string                `json:"summary"`
	Description string                `json:"description"`
	OperationID string                `json:"operationId"`
	Parameters  []Parameter           `json:"parameters"`
	Responses   map[string]Response   `json:"responses"`
	Consumes    []string              `json:"consumes"`
	Produces    []string              `json:"produces"`
	Security    []map[string][]string `json:"security"`
	Deprecated  bool                  `json:"deprecated"`
}

type Parameter struct {
	Name             string  `json:"name"`
	In               string  `json:"in"`
	Description      string  `json:"description"`
	Required         bool    `json:"required"`
	Type             string  `json:"type"`
	Format           string  `json:"format"`
	Schema           *Schema `json:"schema,omitempty"`
	CollectionFormat string  `json:"collectionFormat"`
	Items            *Schema `json:"items,omitempty"`
	Enum             []interface{} `json:"enum,omitempty"`
	Default          interface{}   `json:"default,omitempty"`
}

type Response struct {
	Description string  `json:"description"`
	Schema      *Schema `json:"schema,omitempty"`
}

type Schema struct {
	Ref                  string                 `json:"$ref,omitempty"`
	Type                 string                 `json:"type,omitempty"`
	Format               string                 `json:"format,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Properties           map[string]*Schema     `json:"properties,omitempty"`
	Items                *Schema                `json:"items,omitempty"`
	AdditionalProperties *Schema                `json:"additionalProperties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	Enum                 []interface{}          `json:"enum,omitempty"`
	Example              interface{}            `json:"example,omitempty"`
	VendorExtensions     map[string]interface{} `json:"-"`
}

func (s *Schema) UnmarshalJSON(data []byte) error {
	type Alias Schema
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Extract vendor extensions (x-* fields)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.VendorExtensions = make(map[string]interface{})
	for k, v := range raw {
		if len(k) > 2 && k[:2] == "x-" {
			var val interface{}
			json.Unmarshal(v, &val)
			s.VendorExtensions[k] = val
		}
	}
	return nil
}

type SecurityDefinition struct {
	Type string `json:"type"`
	Name string `json:"name"`
	In   string `json:"in"`
}

func Parse(data []byte) (*Spec, error) {
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	// Extract key order from raw JSON for paths and definitions
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if pathsRaw, ok := raw["paths"]; ok {
		spec.PathOrder = extractKeyOrder(pathsRaw)
	}
	if defsRaw, ok := raw["definitions"]; ok {
		spec.DefinitionOrder = extractKeyOrder(defsRaw)
	}

	return &spec, nil
}

// extractKeyOrder extracts the key order from a JSON object using a streaming decoder.
func extractKeyOrder(data json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(data))
	// Read opening brace
	t, err := dec.Token()
	if err != nil || t != json.Delim('{') {
		return nil
	}
	var keys []string
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			break
		}
		key, ok := t.(string)
		if !ok {
			break
		}
		keys = append(keys, key)
		// Skip the value
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			break
		}
	}
	return keys
}
