package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "swagger-codegen",
	Short: "Dart code generator from Swagger 2.0 specs",
}

func Execute() error {
	return rootCmd.Execute()
}
