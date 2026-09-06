package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/config"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

// A `$ref`-only definition renders as a typedef of its target, so decoding
// through the alias yields the model instead of an empty class.
func TestGenerateDartSDK_RefAlias(t *testing.T) {
	spec, err := swagger.Parse([]byte(testPythonSwagger))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	output := t.TempDir()
	cfg := &config.Config{
		PubName:           "famn_sdk",
		PubVersion:        "0.1.0",
		PubDescription:    "Famn client",
		UseEnumExtension:  true,
		PruneUnusedModels: true,
		KeepModel:         []string{"PetList"},
	}
	if err := New(cfg, spec, output, "dart", "").Generate(); err != nil {
		t.Fatalf("generate Dart SDK: %v", err)
	}
	assertFileContains(t, filepath.Join(output, "lib", "model", "deleted_at.dart"),
		"typedef DeletedAt = NullTime;")
	assertFileContains(t, filepath.Join(output, "lib", "model", "null_time.dart"),
		"class NullTime {")
	assertFileContains(t, filepath.Join(output, "lib", "model", "pet_list.dart"),
		"class PetList {", "final List<Pet> values;")
	assertFileContains(t, filepath.Join(output, "lib", "model", "pet.dart"),
		"DeletedAt? deletedAt;")
}

// An optional list starts null so a request built without it sends nothing;
// a required list starts empty.
func TestGenerateDartSDK_ListDefaults(t *testing.T) {
	spec, err := swagger.Parse([]byte(testPythonSwagger))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	output := t.TempDir()
	cfg := &config.Config{
		PubName:           "famn_sdk",
		PubVersion:        "0.1.0",
		PruneUnusedModels: true,
		KeepModel:         []string{"Kennel"},
	}
	if err := New(cfg, spec, output, "dart", "").Generate(); err != nil {
		t.Fatalf("generate Dart SDK: %v", err)
	}
	assertFileContains(t, filepath.Join(output, "lib", "model", "pet.dart"),
		"List<Pet>? related;")
	assertFileNotContains(t, filepath.Join(output, "lib", "model", "pet.dart"),
		"List<Pet>? related = [];")
	assertFileContains(t, filepath.Join(output, "lib", "model", "kennel.dart"),
		"List<Pet>? pets = [];")
}

func assertFileNotContains(t *testing.T, path string, values ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, value := range values {
		if strings.Contains(string(data), value) {
			t.Errorf("%s must not contain %q", path, value)
		}
	}
}
