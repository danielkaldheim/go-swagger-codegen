package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/config"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

func TestPythonNamesAndTypes(t *testing.T) {
	tests := map[string]string{
		"petID":        "pet_id",
		"class":        "class_",
		"display-name": "display_name",
		"4wdMode":      "value_4wd_mode",
		"list":         "list_",
	}
	for input, want := range tests {
		if got := pythonIdentifier(input); got != want {
			t.Errorf("pythonIdentifier(%q) = %q, want %q", input, got, want)
		}
	}

	typeTests := []struct {
		schema *swagger.Schema
		want   string
	}{
		{&swagger.Schema{Type: "string", Format: "date-time"}, "datetime"},
		{&swagger.Schema{Type: "string", Format: "uuid"}, "UUID"},
		{&swagger.Schema{Type: "array", Items: &swagger.Schema{Ref: "#/definitions/Pet"}}, "list[Pet]"},
		{&swagger.Schema{Type: "object", AdditionalProperties: &swagger.Schema{Type: "integer"}}, "dict[str, int]"},
	}
	for _, test := range typeTests {
		if got := pythonTypeDeclaration(test.schema); got != test.want {
			t.Errorf("pythonTypeDeclaration(%+v) = %q, want %q", test.schema, got, test.want)
		}
	}
}

func TestGeneratePythonSDK(t *testing.T) {
	spec, err := swagger.Parse([]byte(testPythonSwagger))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	output := t.TempDir()
	cfg := &config.Config{
		PackageName:        "Famn SDK",
		PackageVersion:     "0.1.0",
		PackageDescription: "Async Famn client",
		PythonRequires:     ">=3.11",
		UseEnumExtension:   true,
		PruneUnusedModels:  true,
		KeepModel:          []string{"PetList"},
	}
	if err := New(cfg, spec, output, "python", "").Generate(); err != nil {
		t.Fatalf("generate Python SDK: %v", err)
	}

	assertFileContains(t, filepath.Join(output, "pyproject.toml"),
		`name = "famn-sdk"`, `dependencies = ["aiohttp>=3.9.0"]`)
	assertFileContains(t, filepath.Join(output, "src", "famn_sdk", "models.py"),
		"class PetState(str, Enum):", "UNKNOWN = \"\"", "class Pet:",
		"pet_id: int", "list_: str | None", "related: list[Pet] | None",
		`list_=_deserialize(data.get("list"), str | None)`,
		`result["list"] = _serialize(self.list_)`,
		`result["petId"] = _serialize(self.pet_id)`,
		"PetList: TypeAlias = list[Pet]", "class NullTime:",
		"DeletedAt: TypeAlias = NullTime")
	assertFileContains(t, filepath.Join(output, "src", "famn_sdk", "apis.py"),
		"class PetsApi:", "async def create_pet(self, body: Pet)",
		"request_body = body", `path.replace("{pet-id}"`,
		`_add_query(request_query, "tag", tag, "multi")`,
		`auth_names=["Bearer"]`, `auth_names=[]`)
	assertFileContains(t, filepath.Join(output, "src", "famn_sdk", "api_client.py"),
		"session: aiohttp.ClientSession | None = None", "Close only sessions created by this client.",
		`headers["Authorization"] = f"Bearer {self._access_token}"`)
	assertFileContains(t, filepath.Join(output, "src", "famn_sdk", "__init__.py"),
		"from .api_client import ApiClient, FileValue", `__version__ = "0.1.0"`)
	assertFileContains(t, filepath.Join(output, "src", "famn_sdk", "py.typed"), "PEP 561")
}

func assertFileContains(t *testing.T, path string, values ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, value := range values {
		if !strings.Contains(string(data), value) {
			t.Errorf("%s does not contain %q", path, value)
		}
	}
}

const testPythonSwagger = `{
  "swagger": "2.0",
  "info": {"title": "Famn", "version": "1.0.0", "description": "Famn API"},
  "host": "api.famn.example",
  "basePath": "/v1",
  "schemes": ["https"],
  "consumes": ["application/json"],
  "securityDefinitions": {
    "Bearer": {"type": "apiKey", "name": "Authorization", "in": "header"}
  },
  "security": [{"Bearer": []}],
  "definitions": {
    "Pet": {
      "type": "object",
      "required": ["petId"],
      "properties": {
        "petId": {"type": "integer", "format": "int64"},
        "name": {"type": "string"},
        "createdAt": {"type": "string", "format": "date-time"},
		"deletedAt": {"$ref": "#/definitions/DeletedAt"},
		"list": {"type": "string"},
		"related": {"type": "array", "items": {"$ref": "#/definitions/Pet"}},
        "state": {"$ref": "#/definitions/PetState"}
      }
    },
    "PetState": {"type": "string", "enum": ["home", "away"]},
	"PetList": {"type": "array", "items": {"$ref": "#/definitions/Pet"}},
	"Kennel": {
	  "type": "object",
	  "required": ["pets"],
	  "properties": {"pets": {"type": "array", "items": {"$ref": "#/definitions/Pet"}}}
	},
	"DeletedAt": {"$ref": "#/definitions/NullTime"},
	"NullTime": {
	  "type": "object",
	  "properties": {
		"Time": {"type": "string", "format": "date-time"},
		"Valid": {"type": "boolean"}
	  }
	}
  },
  "paths": {
    "/pets": {
      "post": {
        "tags": ["Pets"],
        "operationId": "createPet",
        "parameters": [{"name": "body", "in": "body", "required": true, "schema": {"$ref": "#/definitions/Pet"}}],
        "responses": {"201": {"description": "Created", "schema": {"$ref": "#/definitions/Pet"}}}
      }
    },
    "/pets/{pet-id}": {
      "get": {
        "tags": ["Pets"],
        "operationId": "getPet",
        "security": [],
        "parameters": [
          {"name": "pet-id", "in": "path", "required": true, "type": "integer"},
          {"name": "tag", "in": "query", "required": false, "type": "array", "collectionFormat": "multi", "items": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK", "schema": {"$ref": "#/definitions/Pet"}}}
      }
    }
  }
}`
