package reach

import (
	"fmt"
	"sort"
	"strings"
)

// Mermaid renders a schema as a Mermaid flowchart.
//
// This draws the *schema*, not the edges. The distinction is the whole reason
// the function is safe to call: a schema is a dozen types and a hundred
// declarations whatever the tenant, while the edge set is unbounded and drawing
// it produces a hairball that answers nothing. What a reader wants from a
// picture of a model is "what does this say is possible?", and that question is
// answered entirely by the text they already wrote.
//
// It follows that the picture is a function of the schema. A tenant who
// declares their own namespace gets their own diagram with no further code,
// which is the property that makes this worth having in the portable core
// rather than in one application's admin pages.
//
// Output is deterministic: every collection is sorted before it is written, so
// the same schema always renders the same bytes and a diagram can be committed
// and diffed like anything else.
func (s *Schema) Mermaid() string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	types := sortedKeys(s.Types)
	shared := s.sharedTargetSets()

	for _, name := range types {
		t := s.Types[name]
		b.WriteString("  " + typeNodeID(name) + "[\"" + typeLabel(t) + "\"]\n")
	}

	// Synthetic nodes for target sets that repeat. Without them every grant
	// relation on every type draws its own edge to each of user, key, recovery
	// and role, which is a picture of the repetition rather than of the model.
	setIDs := sortedKeys(shared)
	for _, key := range setIDs {
		b.WriteString("  " + setNodeID(key) + "{{\"" + escapeMermaid(shared[key]) + "\"}}\n")
	}

	for _, name := range types {
		for _, e := range s.edgesOf(name, shared) {
			b.WriteString("  " + typeNodeID(name) + " -->|" + escapeMermaid(e.label) + "| " + e.to + "\n")
		}
	}
	return b.String()
}

// diagramEdge is one drawn arrow: a merged relation label and a target node id.
type diagramEdge struct {
	label string
	to    string
}

// edgesOf collapses a type's relations into drawn edges. Relations sharing a
// target are merged into one arrow, because "can_read, can_manage" on a single
// line says exactly what two parallel arrows say and leaves the shape legible.
func (s *Schema) edgesOf(typeName string, shared map[string]string) []diagramEdge {
	t := s.Types[typeName]

	// Group by destination first, so relations that agree about where they
	// point end up on one arrow.
	byDest := map[string][]string{}
	for rel, targets := range t.Relations {
		dest := setKey(targets)
		if _, isShared := shared[dest]; !isShared {
			// Not a repeated set: draw one arrow per concrete target so the
			// reader sees the real type it points at.
			for _, target := range targets {
				byDest[typeNodeID(target.Type)] = append(byDest[typeNodeID(target.Type)], relationLabel(rel, target))
			}
			continue
		}
		byDest[setNodeID(dest)] = append(byDest[setNodeID(dest)], rel)
	}

	out := make([]diagramEdge, 0, len(byDest))
	for dest, rels := range byDest {
		sort.Strings(rels)
		out = append(out, diagramEdge{label: strings.Join(rels, ", "), to: dest})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].to != out[j].to {
			return out[i].to < out[j].to
		}
		return out[i].label < out[j].label
	})
	return out
}

// sharedTargetSets finds multi-member target sets used by more than one
// relation, and returns each keyed by its canonical form.
//
// The threshold is what keeps the collapse honest. A set used once is drawn as
// itself, because inventing a node for it would hide the single type it names.
func (s *Schema) sharedTargetSets() map[string]string {
	uses := map[string]int{}
	label := map[string]string{}
	for _, t := range s.Types {
		for _, targets := range t.Relations {
			if len(targets) < 2 {
				continue
			}
			key := setKey(targets)
			uses[key]++
			label[key] = setLabel(targets)
		}
	}
	out := map[string]string{}
	for key, n := range uses {
		if n > 1 {
			out[key] = label[key]
		}
	}
	return out
}

// typeLabel is the type name above its rules. The rules belong in the node
// rather than on the arrows: an action is answered at the resource, so drawing
// it as an edge would put it somewhere it does not happen.
func typeLabel(t *TypeDef) string {
	label := escapeMermaid(t.Name)
	for _, action := range sortedKeys(t.Rules) {
		label += "<br/>" + escapeMermaid(action+" = "+t.Rules[action].String())
	}
	return label
}

func relationLabel(rel string, target TargetSpec) string {
	if target.Relation == "" {
		return rel
	}
	return rel + " → #" + target.Relation
}

func setLabel(targets []TargetSpec) string {
	parts := make([]string, 0, len(targets))
	for _, t := range targets {
		parts = append(parts, t.String())
	}
	sort.Strings(parts)
	// A middle dot, not a pipe: Mermaid delimits edge labels with pipes.
	return strings.Join(parts, " · ")
}

func setKey(targets []TargetSpec) string {
	parts := make([]string, 0, len(targets))
	for _, t := range targets {
		parts = append(parts, t.String())
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func typeNodeID(name string) string { return "t_" + sanitiseID(name) }

func setNodeID(key string) string { return "s_" + sanitiseID(key) }

// sanitiseID keeps node ids to characters Mermaid parses unambiguously.
func sanitiseID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// escapeMermaid makes a string safe inside a quoted label.
//
// Two escaping schemes meet here. Labels render as HTML, so the markup
// characters take entities; Mermaid also reads a literal # as the start of its
// own entity escape, and userset targets are full of them. Order matters: the
// markup pass must finish before # is rewritten, or it would go on to escape
// the # that the last two substitutions introduce.
func escapeMermaid(s string) string {
	for _, sub := range [][2]string{
		{"&", "&amp;"},
		{"<", "&lt;"},
		{">", "&gt;"},
		{"#", "#35;"},
		{"\"", "#quot;"},
	} {
		s = strings.ReplaceAll(s, sub[0], sub[1])
	}
	return s
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Summary counts what a schema declares, for a header above the diagram or a
// line in an admin page.
type Summary struct {
	Namespace string
	Actions   int
	Relations int
	Types     int
	Rules     int
	// Wildcards is how many relations accept a star node at either end. It is
	// worth its own count: each one is a place where a single edge can confer
	// reach over everything of a type.
	Wildcards int
	// Transitive is how many relations the walk follows to closure.
	Transitive int
}

func (s Summary) String() string {
	return fmt.Sprintf("%s: %d types, %d relations (%d wildcard, %d transitive), %d actions, %d rules",
		s.Namespace, s.Types, s.Relations, s.Wildcards, s.Transitive, s.Actions, s.Rules)
}

// Describe summarises the schema.
func (s *Schema) Describe() Summary {
	sum := Summary{
		Namespace: s.Namespace,
		Actions:   len(s.Actions),
		Relations: len(s.Relations),
		Types:     len(s.Types),
	}
	for _, d := range s.Relations {
		if d.Wildcard != WildcardNone {
			sum.Wildcards++
		}
		if d.Transitive {
			sum.Transitive++
		}
	}
	for _, t := range s.Types {
		sum.Rules += len(t.Rules)
	}
	return sum
}
