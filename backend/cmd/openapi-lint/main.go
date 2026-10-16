package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
)

func main() {
	var specPath string
	flag.StringVar(&specPath, "spec", "contracts/openapi/openapi.yaml", "Path to OpenAPI spec file")
	flag.Parse()

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading OpenAPI spec %q: %v\n", specPath, err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := doc.Validate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "OpenAPI spec validation failed for %q: %v\n", specPath, err)
		os.Exit(1)
	}

	fmt.Printf("OpenAPI spec %q is valid. (Title: %s, Version: %s, Paths: %d)\n",
		specPath, doc.Info.Title, doc.Info.Version, doc.Paths.Len())
}
