package generator

import (
	"path/filepath"
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
