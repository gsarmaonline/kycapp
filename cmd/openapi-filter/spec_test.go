package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const specPath = "../../docs/openapi.yaml"

type specDoc struct {
	Paths map[string]map[string]yaml.Node `yaml:"paths"`
}

type operation struct {
	OperationID string `yaml:"operationId"`
	RequestBody *struct {
		Content map[string]struct {
			Schema struct {
				Ref string `yaml:"$ref"`
			} `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"requestBody"`
}

// loadIntegrationOps returns every operation the merchant Integration API exposes,
// keyed by "METHOD path".
func loadIntegrationOps(t *testing.T) map[string]operation {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(specPath))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var doc specDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse spec: %v", err)
	}

	ops := map[string]operation{}
	for path, allowMethods := range integrationPaths {
		item, ok := doc.Paths[path]
		if !ok {
			t.Fatalf("spec is missing integration path %s", path)
		}
		for method, node := range item {
			lm := strings.ToLower(method)
			if !httpMethods[lm] {
				continue
			}
			if allowMethods != nil && !containsFold(allowMethods, lm) {
				continue
			}
			var op operation
			if err := node.Decode(&op); err != nil {
				t.Fatalf("decode %s %s: %v", strings.ToUpper(lm), path, err)
			}
			ops[strings.ToUpper(lm)+" "+path] = op
		}
	}
	if len(ops) == 0 {
		t.Fatal("no integration operations found")
	}
	return ops
}

// Generated SDK method names come from operationId. A missing id makes the
// generator fall back to a path-derived name, and a duplicate makes codegen
// ambiguous. Both break merchants, so keep the spec honest here.
func TestIntegrationOperationsHaveUniqueOperationIDs(t *testing.T) {
	ops := loadIntegrationOps(t)

	seen := map[string]string{}
	for key, op := range ops {
		if op.OperationID == "" {
			t.Errorf("%s: missing operationId", key)
			continue
		}
		if prev, dup := seen[op.OperationID]; dup {
			t.Errorf("%s: operationId %q already used by %s", key, op.OperationID, prev)
			continue
		}
		seen[op.OperationID] = key
	}
}

// Inline request bodies generate anonymous types that the SDK cannot import or
// re-export. Every integration request body must point at components/schemas.
func TestIntegrationRequestBodiesAreNamedSchemas(t *testing.T) {
	ops := loadIntegrationOps(t)

	for key, op := range ops {
		if op.RequestBody == nil {
			continue
		}
		content, ok := op.RequestBody.Content["application/json"]
		if !ok {
			continue
		}
		if content.Schema.Ref == "" {
			t.Errorf("%s: inline request body; move it to components/schemas and $ref it", key)
			continue
		}
		if !strings.HasPrefix(content.Schema.Ref, "#/components/schemas/") {
			t.Errorf("%s: request body $ref %q must target #/components/schemas/", key, content.Schema.Ref)
		}
	}
}
