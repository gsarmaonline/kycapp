package reach

import (
	"strings"
	"testing"
)

const diagramSchema = `
namespace docs

action read, write

relation member_of : transitive
relation parent    : direct
relation viewer    : direct, wildcard subject
relation editor    : direct

type user
type group
  relation member_of -> user | group#member_of

type folder
  relation viewer -> user | group#member_of
  rule read = viewer

type document
  relation parent -> folder
  relation viewer -> user | group#member_of
  relation editor -> user | group#member_of
  rule read  = viewer + editor + parent->read
  rule write = editor
`

func TestMermaidIsDeterministic(t *testing.T) {
	s, err := Parse(diagramSchema)
	if err != nil {
		t.Fatal(err)
	}
	first := s.Mermaid()
	for i := 0; i < 20; i++ {
		// Map iteration is randomised, so twenty runs is enough to catch an
		// unsorted collection. A diagram that is not byte-stable cannot be
		// committed and diffed, which is most of the point of rendering one.
		if got := s.Mermaid(); got != first {
			t.Fatalf("run %d differs:\n%s\n---\n%s", i, first, got)
		}
	}
}

func TestMermaidDrawsTypesRulesAndRelations(t *testing.T) {
	s, err := Parse(diagramSchema)
	if err != nil {
		t.Fatal(err)
	}
	out := s.Mermaid()

	for _, want := range []string{
		"flowchart LR",
		`t_document["document`,
		"read = viewer + editor + parent-&gt;read",
		"t_document -->|parent| t_folder",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("diagram is missing %q:\n%s", want, out)
		}
	}
}

func TestMermaidCollapsesRepeatedTargetSetsOnly(t *testing.T) {
	s, err := Parse(diagramSchema)
	if err != nil {
		t.Fatal(err)
	}
	out := s.Mermaid()

	// user | group#member_of is used by four relations, so it becomes one node
	// and the relations sharing it merge onto a single arrow.
	if !strings.Contains(out, `{{"group#35;member_of · user"}}`) {
		t.Errorf("repeated target set was not collapsed:\n%s", out)
	}
	if !strings.Contains(out, "|editor, viewer|") {
		t.Errorf("relations sharing a target were not merged:\n%s", out)
	}
	// parent -> folder is used once, so it must still name the real type
	// rather than hide behind a synthetic node.
	if !strings.Contains(out, "t_document -->|parent| t_folder") {
		t.Errorf("a single-target relation was collapsed:\n%s", out)
	}
}

func TestMermaidEscapesUsersetHashes(t *testing.T) {
	s, err := Parse(diagramSchema)
	if err != nil {
		t.Fatal(err)
	}
	// A bare # starts an entity escape in Mermaid, and userset targets are full
	// of them. Every one must arrive escaped or the label renders as garbage.
	for _, line := range strings.Split(s.Mermaid(), "\n") {
		rest := line
		for {
			i := strings.Index(rest, "#")
			if i < 0 {
				break
			}
			rest = rest[i+1:]
			if !strings.HasPrefix(rest, "35;") && !strings.HasPrefix(rest, "quot;") {
				t.Errorf("unescaped # in %q", line)
				break
			}
		}
	}
}

func TestDescribeCounts(t *testing.T) {
	s, err := Parse(diagramSchema)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Describe()
	if got.Types != 4 {
		t.Errorf("Types = %d, wanted 4", got.Types)
	}
	if got.Relations != 4 {
		t.Errorf("Relations = %d, wanted 4", got.Relations)
	}
	if got.Rules != 3 {
		t.Errorf("Rules = %d, wanted 3", got.Rules)
	}
	if got.Transitive != 1 {
		t.Errorf("Transitive = %d, wanted 1", got.Transitive)
	}
	if got.Wildcards != 1 {
		t.Errorf("Wildcards = %d, wanted 1", got.Wildcards)
	}
}
