package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/config"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/generator"
	"gitlab.crudus.no/crudus/swagger-codegen/pkg/swagger"
)

var (
	inputFile  string
	outputDir  string
	configFile string
)

func init() {
	generateCmd.Flags().StringVarP(&inputFile, "input", "i", "", "Path to swagger.json (required)")
	generateCmd.Flags().StringVarP(&outputDir, "output", "o", "output", "Output directory")
	generateCmd.Flags().StringVarP(&configFile, "config", "c", "", "Path to config.json")
	generateCmd.MarkFlagRequired("input")
	rootCmd.AddCommand(generateCmd)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Dart client code from a Swagger 2.0 spec",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		specData, err := os.ReadFile(inputFile)
		if err != nil {
			return fmt.Errorf("reading swagger spec: %w", err)
		}

		spec, err := swagger.Parse(specData)
		if err != nil {
			return fmt.Errorf("parsing swagger spec: %w", err)
		}

		gen := generator.New(cfg, spec, outputDir)
		if err := gen.Generate(); err != nil {
			return fmt.Errorf("generating code: %w", err)
		}

		fmt.Printf("Generated Dart client in %s\n", outputDir)
		return nil
	},
}
