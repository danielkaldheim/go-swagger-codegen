package main

import (
	"fmt"
	"os"

	"gitlab.crudus.no/crudus/swagger-codegen/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
