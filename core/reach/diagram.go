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
	g := s.Graph()

	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, n := range g.Nodes {
		if n.Kind == GraphKindSet {
			// A brace shape marks the synthetic nodes, so a reader can tell a
			// declared type from a collapsed target set at a glance.
			b.WriteString("  " + n.ID + "{{\"" + escapeMermaid(n.Label) + "\"}}\n")
			continue
		}
		label := escapeMermaid(n.Label)
		for _, r := range n.Rules {
			label += "<br/>" + escapeMermaid(r.Action+" = "+r.Expr)
		}
		b.WriteString("  " + n.ID + "[\"" + label + "\"]\n")
	}
	for _, e := range g.Edges {
		b.WriteString("  " + e.From + " -->|" + escapeMermaid(e.Label) + "| " + e.To + "\n")
	}
	return b.String()
}

// Graph is a schema as nodes and edges, for a renderer that lays out its own
// picture rather than consuming one.
//
// Mermaid renders from this, so the two can never disagree about which nodes
// exist or which arrows were merged. Anything that draws a schema should start
// here; Mermaid is one such renderer that happens to live in the same file.
type Graph struct {
	Namespace string      `json:"namespace"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	Summary   Summary     `json:"summary"`
	// Shapes groups types that are indistinguishable except by name.
	Shapes []Shape `json:"shapes,omitempty"`
}

// Shape is a set of types that declare the same relations pointing at the same
// places and answer the same actions the same way.
//
// It is computed here rather than by whatever draws the graph, so the answer
// cannot differ between renderers. The definition has two halves and both
// matter: types that share a container look identical in a picture while
// answering entirely different actions, and comparing arrows alone reports a
// larger, wrong number.
type Shape struct {
	Types []string `json:"types"`
	// Rules is the shared rule set, as "action = expr", already sorted. Empty
	// for types that answer nothing, which are structural rather than governed.
	Rules []string `json:"rules,omitempty"`
}

// GraphKind distinguishes a declared type from a node this package invented.
type GraphKind string

const (
	// GraphKindType is a type the schema declares.
	GraphKindType GraphKind = "type"
	// GraphKindSet is a synthetic node standing for a target set that several
	// relations share. It exists in the picture, never in the schema.
	GraphKindSet GraphKind = "set"
)

// GraphNode is one drawn node.
type GraphNode struct {
	ID    string      `json:"id"`
	Kind  GraphKind   `json:"kind"`
	Label string      `json:"label"`
	Rules []GraphRule `json:"rules,omitempty"`
	// Members is the target set a synthetic node stands for, already sorted.
	// Empty for a declared type.
	Members []string `json:"members,omitempty"`
}

// GraphRule is one action and the expression that answers it.
type GraphRule struct {
	Action string `json:"action"`
	Expr   string `json:"expr"`
}

// GraphEdge is one drawn arrow. Relations that agree about where they point
// share an edge, so Label may name several.
type GraphEdge struct {
	ID        string   `json:"id"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Label     string   `json:"label"`
	Relations []string `json:"relations"`
}

// Graph builds the drawable form of the schema.
//
// Everything is sorted, so the same schema always produces the same graph and a
// generated file can be committed and diffed like source.
func (s *Schema) Graph() Graph {
	types := sortedKeys(s.Types)
	shared := s.sharedTargetSets()

	g := Graph{
		Namespace: s.Namespace,
		Nodes:     make([]GraphNode, 0, len(types)+len(shared)),
		Edges:     []GraphEdge{},
		Summary:   s.Describe(),
	}

	for _, name := range types {
		t := s.Types[name]
		node := GraphNode{ID: typeNodeID(name), Kind: GraphKindType, Label: name}
		for _, action := range sortedKeys(t.Rules) {
			node.Rules = append(node.Rules, GraphRule{Action: action, Expr: t.Rules[action].String()})
		}
		g.Nodes = append(g.Nodes, node)
	}

	for _, key := range sortedKeys(shared) {
		g.Nodes = append(g.Nodes, GraphNode{
			ID:      setNodeID(key),
			Kind:    GraphKindSet,
			Label:   shared[key],
			Members: strings.Split(key, "|"),
		})
	}

	for _, name := range types {
		from := typeNodeID(name)
		for _, e := range s.edgesOf(name, shared) {
			g.Edges = append(g.Edges, GraphEdge{
				ID:        from + "__" + e.to + "__" + sanitiseID(e.label),
				From:      from,
				To:        e.to,
				Label:     e.label,
				Relations: strings.Split(e.label, ", "),
			})
		}
	}
	g.Shapes = g.shapes()
	return g
}

// shapes groups types by what makes them indistinguishable.
//
// Only groups of more than one are returned: a type that is its own shape is
// not a repetition, and reporting it as one would bury the finding in noise.
func (g Graph) shapes() []Shape {
	arrows := map[string][]string{}
	for _, e := range g.Edges {
		arrows[e.From] = append(arrows[e.From], e.Label+" -> "+e.To)
	}

	byKey := map[string]*Shape{}
	var keys []string
	for _, n := range g.Nodes {
		if n.Kind != GraphKindType {
			continue
		}
		rules := make([]string, 0, len(n.Rules))
		for _, r := range n.Rules {
			rules = append(rules, r.Action+" = "+r.Expr)
		}
		sort.Strings(rules)
		out := append([]string(nil), arrows[n.ID]...)
		sort.Strings(out)

		key := strings.Join(rules, ";") + "|" + strings.Join(out, ";")
		if existing, ok := byKey[key]; ok {
			existing.Types = append(existing.Types, n.Label)
			continue
		}
		byKey[key] = &Shape{Types: []string{n.Label}, Rules: rules}
		keys = append(keys, key)
	}

	out := make([]Shape, 0, len(keys))
	for _, key := range keys {
		if s := byKey[key]; len(s.Types) > 1 {
			sort.Strings(s.Types)
			out = append(out, *s)
		}
	}
	// Largest first, then alphabetically, so the ordering does not depend on
	// which type happened to be visited first.
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Types) != len(out[j].Types) {
			return len(out[i].Types) > len(out[j].Types)
		}
		return out[i].Types[0] < out[j].Types[0]
	})
	return out
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
	Namespace string `json:"namespace"`
	Actions   int    `json:"actions"`
	Relations int    `json:"relations"`
	Types     int    `json:"types"`
	Rules     int    `json:"rules"`
	// Wildcards is how many relations accept a star node at either end. It is
	// worth its own count: each one is a place where a single edge can confer
	// reach over everything of a type.
	Wildcards int `json:"wildcards"`
	// Transitive is how many relations the walk follows to closure.
	Transitive int `json:"transitive"`
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
