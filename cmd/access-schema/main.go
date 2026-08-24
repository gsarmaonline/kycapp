// Command access-schema writes KYC's authorisation schema as a drawable graph.
//
// The schema is a compile-time constant, so this is a build step rather than an
// endpoint. Serving it would mean answering an authorisation question about the
// authorisation model, and getting a static file wrong is a stale diagram
// rather than a disclosure. It also means the public documentation can render
// the graph without a session.
//
// The output lands in web/public alongside the OpenAPI spec, which is already
// what `make sdk-check` diffs, so a schema edited without regenerating fails CI
// the same way a spec does.
//
// A merchant's own schema cannot work this way. Theirs is tenant data, read at
// request time and gated like anything else.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/gsarmaonline/kyc/internal/accessmodel"
)

func main() {
	out := flag.String("out", "web/public/access-schema.json", "file to write")
	flag.Parse()

	schema, err := accessmodel.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "access-schema:", err)
		os.Exit(1)
	}

	// Indented and newline-terminated so the committed file reads as source and
	// a diff points at the declaration that changed rather than one long line.
	body, err := json.MarshalIndent(schema.Graph(), "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "access-schema:", err)
		os.Exit(1)
	}
	body = append(body, '\n')

	if err := os.WriteFile(*out, body, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "access-schema:", err)
		os.Exit(1)
	}
	g := schema.Graph()
	fmt.Printf("wrote %s (%d nodes, %d edges)\n", *out, len(g.Nodes), len(g.Edges))
}
