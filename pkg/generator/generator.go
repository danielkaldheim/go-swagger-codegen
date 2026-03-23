package generator

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/config"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

type Generator struct {
	config    *config.Config
	spec      *swagger.Spec
	outputDir string
	models    []codegen.ModelData
	apis      []codegen.ApiData
	auth      []codegen.AuthMethodData
	tmpl      *template.Template
}

func New(cfg *config.Config, spec *swagger.Spec, outputDir string) *Generator {
	return &Generator{
		config:    cfg,
		spec:      spec,
		outputDir: outputDir,
	}
}

func (g *Generator) Generate() error {
	// Build model data
	g.models = codegen.BuildModels(g.spec.Definitions, g.spec.Definitions, g.config.UseEnumExtension, g.spec.DefinitionOrder)

	// Build API data
	g.apis = codegen.BuildApis(g.spec)

	// Build auth methods
	g.auth = codegen.BuildAuthMethods(g.spec.SecurityDefinitions)

	// Load templates
	if err := g.loadTemplates(); err != nil {
		return fmt.Errorf("loading templates: %w", err)
	}

	// Create output directories
	dirs := []string{
		filepath.Join(g.outputDir, "lib"),
		filepath.Join(g.outputDir, "lib", "model"),
		filepath.Join(g.outputDir, "lib", "api"),
		filepath.Join(g.outputDir, "lib", "auth"),
		filepath.Join(g.outputDir, "docs"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Generate model files first (before supporting files, so base_model.dart
	// from supporting files overwrites any model with the same filename)
	if err := g.generateModels(); err != nil {
		return fmt.Errorf("generating models: %w", err)
	}

	// Generate API files
	if err := g.generateApis(); err != nil {
		return fmt.Errorf("generating APIs: %w", err)
	}

	// Generate supporting files last (overwrites base_model.dart if a
	// BaseModel definition exists in the swagger spec)
	if err := g.generateSupportingFiles(); err != nil {
		return fmt.Errorf("generating supporting files: %w", err)
	}

	return nil
}

func (g *Generator) loadTemplates() error {
	funcMap := buildFuncMap()

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/dart/*.go.tmpl",
		"templates/dart/auth/*.go.tmpl",
	)
	if err != nil {
		return err
	}
	g.tmpl = tmpl
	return nil
}

func (g *Generator) globalData() GlobalData {
	basePath := "https://" + g.spec.Host + g.spec.BasePath
	if len(g.spec.Schemes) > 0 {
		basePath = g.spec.Schemes[0] + "://" + g.spec.Host + g.spec.BasePath
	}

	return GlobalData{
		PubName:        g.config.PubName,
		PubVersion:     g.config.PubVersion,
		PubDescription: g.config.PubDescription,
		BrowserClient:  g.config.BrowserClient,
		BasePath:       basePath,
		AppVersion:     g.spec.Info.Version,
		AppDescription: g.spec.Info.Description,
		ApiDocPath:     "docs/",
		ModelDocPath:   "docs/",
		Models:         g.models,
		Apis:           g.apis,
		AuthMethods:    g.auth,
		ApiInfo:        &ApiInfo{Apis: g.apis},
	}
}

func (g *Generator) renderToFile(tmplName, outputPath string, data any) error {
	var buf bytes.Buffer
	if err := g.tmpl.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return fmt.Errorf("executing template %s: %w", tmplName, err)
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0644)
}

// renderToFileNoTrailingNewline renders a template and strips trailing newlines
// to match Java Mustache output for certain templates.
func (g *Generator) renderToFileNoTrailingNewline(tmplName, outputPath string, data any) error {
	var buf bytes.Buffer
	if err := g.tmpl.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return fmt.Errorf("executing template %s: %w", tmplName, err)
	}
	out := bytes.TrimRight(buf.Bytes(), "\n")
	return os.WriteFile(outputPath, out, 0644)
}

func (g *Generator) generateSupportingFiles() error {
	gd := g.globalData()

	supportingFiles := []struct {
		template       string
		path           string
		stripTrailing  bool
	}{
		{"pubspec.go.tmpl", filepath.Join(g.outputDir, "pubspec.yaml"), true},
		{"analysis_options.go.tmpl", filepath.Join(g.outputDir, ".analysis_options"), true},
		{"gitignore.go.tmpl", filepath.Join(g.outputDir, ".gitignore"), true},
		{"README.go.tmpl", filepath.Join(g.outputDir, "README.md"), false},
		{"apilib.go.tmpl", filepath.Join(g.outputDir, "lib", "api.dart"), true},
		{"api_client.go.tmpl", filepath.Join(g.outputDir, "lib", "api_client.dart"), false},
		{"api_exception.go.tmpl", filepath.Join(g.outputDir, "lib", "api_exception.dart"), false},
		{"api_helper.go.tmpl", filepath.Join(g.outputDir, "lib", "api_helper.dart"), false},
		{"provider.go.tmpl", filepath.Join(g.outputDir, "lib", "api_provider.dart"), false},
		{"basemodel.go.tmpl", filepath.Join(g.outputDir, "lib", "model", "base_model.dart"), false},
		{"authentication.go.tmpl", filepath.Join(g.outputDir, "lib", "auth", "authentication.dart"), false},
		{"http_basic_auth.go.tmpl", filepath.Join(g.outputDir, "lib", "auth", "http_basic_auth.dart"), true},
		{"api_key_auth.go.tmpl", filepath.Join(g.outputDir, "lib", "auth", "api_key_auth.dart"), false},
		{"oauth.go.tmpl", filepath.Join(g.outputDir, "lib", "auth", "oauth.dart"), false},
	}

	for _, sf := range supportingFiles {
		var err error
		if sf.stripTrailing {
			err = g.renderToFileNoTrailingNewline(sf.template, sf.path, gd)
		} else {
			err = g.renderToFile(sf.template, sf.path, gd)
		}
		if err != nil {
			return fmt.Errorf("generating %s: %w", sf.path, err)
		}
	}

	return nil
}

func (g *Generator) generateModels() error {
	for _, model := range g.models {
		data := ModelTemplateData{
			PubName: g.config.PubName,
			Models:  []ModelWrapper{{Model: model}},
		}
		outputPath := filepath.Join(g.outputDir, "lib", "model", model.ClassFilename+".dart")
		if err := g.renderToFile("model.go.tmpl", outputPath, data); err != nil {
			return fmt.Errorf("generating model %s: %w", model.Classname, err)
		}
	}
	return nil
}

func (g *Generator) generateApis() error {
	for _, api := range g.apis {
		data := ApiTemplateData{
			PubName:    g.config.PubName,
			Operations: &api,
		}
		outputPath := filepath.Join(g.outputDir, "lib", "api", api.ClassFilename+".dart")
		if err := g.renderToFile("api.go.tmpl", outputPath, data); err != nil {
			return fmt.Errorf("generating API %s: %w", api.Classname, err)
		}
	}
	return nil
}
