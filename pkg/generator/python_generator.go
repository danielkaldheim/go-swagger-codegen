package generator

import (
	"fmt"
	"os"
	"path/filepath"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
)

func (g *Generator) generatePython() error {
	// Reuse the established selection pass so excludeApi, excludeModel,
	// pruneUnusedModels, and keepModel mean the same thing for every language.
	g.models = codegen.BuildModels(g.spec.Definitions, g.spec.Definitions, g.config.UseEnumExtension, g.spec.DefinitionOrder, g.config.NativeEnums)
	g.apis = codegen.BuildApis(g.spec)
	if len(g.config.ExcludeModel) > 0 {
		g.models = filterModels(g.models, g.config.ExcludeModel)
	}
	if len(g.config.ExcludeApi) > 0 {
		g.apis = filterApis(g.apis, g.config.ExcludeApi)
	}
	if g.config.PruneUnusedModels {
		g.models = codegen.PruneUnusedModels(g.models, g.apis, g.config.KeepModel)
	}

	packageName := g.config.PackageName
	if packageName == "" {
		packageName = g.config.PubName
	}
	packageName = pythonPackageName(packageName)
	packageVersion := g.config.PackageVersion
	if packageVersion == "" {
		packageVersion = g.config.PubVersion
	}
	packageDescription := g.config.PackageDescription
	if packageDescription == "" {
		packageDescription = g.config.PubDescription
	}
	pythonRequires := g.config.PythonRequires
	if pythonRequires == "" {
		pythonRequires = ">=3.11"
	}

	data := buildPythonData(g.spec, pythonConfig{
		PackageName:        packageName,
		DistributionName:   g.config.DistributionName,
		PackageVersion:     packageVersion,
		PackageDescription: packageDescription,
		PythonRequires:     pythonRequires,
		NativeEnums:        g.config.NativeEnums,
		UseEnumExtension:   g.config.UseEnumExtension,
	}, g.models, g.apis)

	if err := g.loadTemplates(); err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}
	packageDir := filepath.Join(g.outputDir, "src", packageName)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		return fmt.Errorf("creating Python package directory %s: %w", packageDir, err)
	}

	files := []struct {
		template string
		path     string
	}{
		{"pyproject.go.tmpl", filepath.Join(g.outputDir, "pyproject.toml")},
		{"README.go.tmpl", filepath.Join(g.outputDir, "README.md")},
		{"init.go.tmpl", filepath.Join(packageDir, "__init__.py")},
		{"exceptions.go.tmpl", filepath.Join(packageDir, "exceptions.py")},
		{"models.go.tmpl", filepath.Join(packageDir, "models.py")},
		{"api_client.go.tmpl", filepath.Join(packageDir, "api_client.py")},
		{"apis.go.tmpl", filepath.Join(packageDir, "apis.py")},
		{"pytyped.go.tmpl", filepath.Join(packageDir, "py.typed")},
	}
	for _, file := range files {
		if err := g.renderToFile(file.template, file.path, data); err != nil {
			return fmt.Errorf("generating %s: %w", file.path, err)
		}
	}
	return nil
}
