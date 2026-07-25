package generator

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gitlab.crudus.no/crudus/swagger-codegen/pkg/codegen"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/config"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

type Generator struct {
	config    *config.Config
	spec      *swagger.Spec
	outputDir string
	language  string
	variant   string
	models    []codegen.ModelData
	apis      []codegen.ApiData
	auth      []codegen.AuthMethodData
	tmpl      *template.Template
}

func New(cfg *config.Config, spec *swagger.Spec, outputDir, language, variant string) *Generator {
	return &Generator{
		config:    cfg,
		spec:      spec,
		outputDir: outputDir,
		language:  language,
		variant:   variant,
	}
}

// templateDir returns the template subdirectory name for the selected language/variant.
func (g *Generator) templateDir() string {
	if g.variant != "" {
		return g.language + "-" + g.variant
	}
	return g.language
}

func (g *Generator) Generate() error {
	// Build model data
	g.models = codegen.BuildModels(g.spec.Definitions, g.spec.Definitions, g.config.UseEnumExtension, g.spec.DefinitionOrder, g.config.NativeEnums)

	// Build API data
	g.apis = codegen.BuildApis(g.spec)

	// Build auth methods
	g.auth = codegen.BuildAuthMethods(g.spec.SecurityDefinitions)

	// Filter excluded models
	if len(g.config.ExcludeModel) > 0 {
		g.models = filterModels(g.models, g.config.ExcludeModel)
	}

	// Filter excluded APIs
	if len(g.config.ExcludeApi) > 0 {
		g.apis = filterApis(g.apis, g.config.ExcludeApi)
	}

	// Prune models not reachable from the (post-filter) API surface. This runs
	// after the exclude filters so removing an API also drops the models only
	// that API needed, and manually excluded models are never re-introduced.
	if g.config.PruneUnusedModels {
		g.models = codegen.PruneUnusedModels(g.models, g.apis, g.config.KeepModel)
	}

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

	// Generate documentation
	if err := g.generateDocs(); err != nil {
		return fmt.Errorf("generating docs: %w", err)
	}

	return nil
}

func (g *Generator) loadTemplates() error {
	dir := "templates/" + g.templateDir()

	// Validate that the selected template directory exists
	if _, err := fs.Stat(templateFS, dir); err != nil {
		available, _ := listAvailableLanguages()
		return fmt.Errorf("template directory %q not found; available: %s",
			g.templateDir(), strings.Join(available, ", "))
	}

	// Create a sub-filesystem rooted at the selected language directory
	subFS, err := fs.Sub(templateFS, dir)
	if err != nil {
		return fmt.Errorf("creating sub-filesystem for %s: %w", dir, err)
	}

	funcMap := buildFuncMap()

	// Parse top-level templates
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(subFS, "*.go.tmpl")
	if err != nil {
		return fmt.Errorf("parsing templates in %s: %w", dir, err)
	}

	// Parse auth/ subdirectory if it exists (not all languages may have auth templates)
	if _, statErr := fs.Stat(subFS, "auth"); statErr == nil {
		tmpl, err = tmpl.ParseFS(subFS, "auth/*.go.tmpl")
		if err != nil {
			return fmt.Errorf("parsing auth templates in %s: %w", dir, err)
		}
	}

	g.tmpl = tmpl
	return nil
}

func (g *Generator) globalData() GlobalData {
	basePath := "https://" + g.spec.Host + g.spec.BasePath
	if len(g.spec.Schemes) > 0 {
		basePath = g.spec.Schemes[0] + "://" + g.spec.Host + g.spec.BasePath
	}
	basePath = strings.TrimRight(basePath, "/")

	return GlobalData{
		PubName:        g.config.PubName,
		PubVersion:     g.config.PubVersion,
		PubDescription: g.config.PubDescription,
		BrowserClient:  g.config.BrowserClient,
		BasePath:       basePath,
		AppVersion:     g.spec.Info.Version,
		AppDescription: g.spec.Info.Description,
		InfoEmail:      g.spec.Info.Contact.Email,
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

func filterModels(models []codegen.ModelData, exclude []string) []codegen.ModelData {
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}
	filtered := make([]codegen.ModelData, 0, len(models))
	for _, m := range models {
		if !excludeSet[m.Classname] {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func filterApis(apis []codegen.ApiData, exclude []string) []codegen.ApiData {
	excludeSet := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		excludeSet[name] = true
	}
	filtered := make([]codegen.ApiData, 0, len(apis))
	for _, a := range apis {
		if !excludeSet[a.Classname] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
