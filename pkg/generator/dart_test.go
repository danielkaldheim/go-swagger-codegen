package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/config"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

func TestRenderDartMultipartOmitsOptionalTextInsteadOfSendingNull(t *testing.T) {
	rendered := renderOperations([]codegen.OperationData{{
		Nickname: "sendPhoto", HttpMethod: "POST", Path: "/photo", HasFormParams: true,
		FormParams: []codegen.ParamData{
			{ParamName: "turnId", BaseName: "turnId", Required: true, NotFile: true},
			{ParamName: "message", BaseName: "message", NotFile: true},
			{ParamName: "photo", BaseName: "photo", IsFile: true},
		},
		Consumes: []codegen.MediaType{{MediaType: "multipart/form-data"}},
	}})
	for _, expected := range []string{
		"mp.fields['turnId'] = parameterToString(turnId);",
		"if (message != null) {",
		"mp.fields['message'] = parameterToString(message);",
		"if(photo != null) {",
		"mp.files.add(photo);",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("multipart renderer omitted %q:\n%s", expected, rendered)
		}
	}
	if strings.Contains(rendered, "mp.fields['message'] = parameterToString(message);\n      \n") {
		t.Fatalf("optional multipart message was emitted without its null guard:\n%s", rendered)
	}
	if strings.Contains(rendered, "mp.fields['photo']") {
		t.Fatalf("multipart file was also emitted as a bogus text field:\n%s", rendered)
	}
}

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
	assertGeneratedDocsWhitespace(t, filepath.Join(output, "docs"))
}

func assertGeneratedDocsWhitespace(t *testing.T, docsDir string) {
	t.Helper()
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		t.Fatalf("read generated docs: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(docsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.HasSuffix(content, "\n\n") {
			t.Errorf("%s ends with extra blank lines", path)
		}
		for lineNumber, line := range strings.Split(content, "\n") {
			if strings.TrimRight(line, " \t") != line {
				t.Errorf("%s:%d has trailing whitespace", path, lineNumber+1)
			}
		}
	}
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

// The SDK is a workspace package: never published, and its base types come from
// crudus_common, as a path dependency when the config names a checkout.
func TestGenerateDartSDK_PubspecUsesCrudusCommon(t *testing.T) {
	spec, err := swagger.Parse([]byte(testPythonSwagger))
	if err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{"hosted", "", "  crudus_common: '>=1.0.0'\n"},
		{"path", "../../packages/crudus_common", "  crudus_common:\n    path: ../../packages/crudus_common\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := t.TempDir()
			cfg := &config.Config{PubName: "famn_sdk", PubVersion: "0.1.0", PubDescription: "Famn client", CrudusCommonPath: tc.path, UseEnumExtension: true}
			if err := New(cfg, spec, output, "dart", "").Generate(); err != nil {
				t.Fatalf("generate Dart SDK: %v", err)
			}
			assertFileContains(t, filepath.Join(output, "pubspec.yaml"), "publish_to: none\n", tc.want, "environment:\n  sdk:")
			for _, gone := range []string{filepath.Join("lib", "api_exception.dart"), filepath.Join("lib", "model", "base_model.dart")} {
				if _, err := os.Stat(filepath.Join(output, gone)); err == nil {
					t.Errorf("%s is still generated; crudus_common replaces it", gone)
				}
			}
		})
	}
}
