package generator

import (
	"embed"
	"io/fs"
	"sort"
)

//go:embed templates
var templateFS embed.FS

// listAvailableLanguages returns the names of all directories under templates/,
// representing available language (or language-variant) template sets.
func listAvailableLanguages() ([]string, error) {
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		return nil, err
	}
	var langs []string
	for _, e := range entries {
		if e.IsDir() {
			langs = append(langs, e.Name())
		}
	}
	sort.Strings(langs)
	return langs, nil
}
