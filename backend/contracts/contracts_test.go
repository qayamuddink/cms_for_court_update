package contracts

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestOpenAPISpecValidation(t *testing.T) {
	specPath := filepath.Join("openapi", "openapi.yaml")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("failed to load OpenAPI spec from %s: %v", specPath, err)
	}

	ctx := context.Background()
	if err := doc.Validate(ctx); err != nil {
		t.Fatalf("OpenAPI specification validation failed: %v", err)
	}

	if doc.Info == nil || doc.Info.Title == "" {
		t.Errorf("expected OpenAPI info title to be non-empty")
	}

	// Verify /healthz is specified
	healthPath := doc.Paths.Find("/healthz")
	if healthPath == nil || healthPath.Get == nil {
		t.Errorf("expected GET /healthz path to exist in OpenAPI spec")
	}

	// Verify ErrorResponse and HealthResponse schemas exist
	if _, ok := doc.Components.Schemas["HealthResponse"]; !ok {
		t.Errorf("expected HealthResponse schema in components")
	}
	if _, ok := doc.Components.Schemas["ErrorResponse"]; !ok {
		t.Errorf("expected ErrorResponse schema in components")
	}
}
