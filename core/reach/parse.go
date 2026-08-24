package reach

import (
	"fmt"
	"strings"
)

// Parse reads the schema language and returns a validated Schema.
//
//	namespace org:acme
//
//	action read, write, delete, share
//
//	relation member : transitive          // grouping
//	relation parent : transitive, wildcard none
//	relation viewer : direct, wildcard both
//
//	type folder
//	  relation parent -> folder
//	  relation editor -> user | group#member
//	  rule read  = viewer + editor + parent->read
//	  rule write = editor + parent->write
//
// A line comment starts at // anywhere, or at a # that follows whitespace. The
// second form lets a userset target such as group#member stay unambiguous.
func Parse(src string) (*Schema, error) {
	p := &parser{schema: NewSchema("")}
	for i, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(stripComment(raw))
		if line == "" {
			continue
		}
		if err := p.line(line); err != nil {
			return nil, fmt.Errorf("%w: line %d: %s", ErrInvalidSchema, i+1, err)
		}
	}
	if err := p.resolve(); err != nil {
		return nil, err
	}
	if err := p.schema.Validate(); err != nil {
		return nil, err
	}
	return p.schema, nil
}

// MustParse is Parse for schemas defined as constants in code and for tests.
func MustParse(src string) *Schema {
	s, err := Parse(src)
	if err != nil {
		panic(err)
	}
	return s
}

func stripComment(s string) string {
	if i := strings.Index(s, "//"); i >= 0 {
		s = s[:i]
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' {
			return s[:i]
		}
	}
	return s
}

type parser struct {
	schema  *Schema
	current *TypeDef
}

func (p *parser) line(line string) error {
	keyword, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)

	switch keyword {
	case "namespace":
		if rest == "" {
			return fmt.Errorf("namespace needs a name")
		}
		p.schema.Namespace = rest
		p.current = nil
		return nil

	case "action":
		for _, a := range strings.Split(rest, ",") {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			p.schema.Actions = append(p.schema.Actions, a)
		}
		p.current = nil
		return nil

	case "type":
		if rest == "" {
			return fmt.Errorf("type needs a name")
		}
		if _, dup := p.schema.Types[rest]; dup {
			return fmt.Errorf("type %q is declared twice", rest)
		}
		p.current = &TypeDef{
			Name:      rest,
			Relations: map[string][]TargetSpec{},
			Rules:     map[string]Expr{},
		}
		p.schema.Types[rest] = p.current
		return nil

	case "relation":
		// A namespace declaration carries a colon; a type's carries an arrow.
		// The two forms never collide, so no indentation rule is needed.
		if strings.Contains(rest, "->") {
			return p.typeRelation(rest)
		}
		return p.namespaceRelation(rest)

	case "rule":
		return p.rule(rest)

	default:
		return fmt.Errorf("unknown keyword %q", keyword)
	}
}

func (p *parser) namespaceRelation(rest string) error {
	name, mods, ok := strings.Cut(rest, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("relation needs a name")
	}
	if _, dup := p.schema.Relations[name]; dup {
		return fmt.Errorf("relation %q is declared twice", name)
	}
	def := RelationDef{Name: name}
	if !ok {
		p.schema.Relations[name] = def
		return nil
	}

	for _, mod := range strings.Split(mods, ",") {
		mod = strings.TrimSpace(mod)
		if mod == "" {
			continue
		}
		word, arg, _ := strings.Cut(mod, " ")
		switch word {
		case "direct":
			def.Transitive = false
		case "transitive":
			def.Transitive = true
		case "wildcard":
			w, err := parseWildcard(strings.TrimSpace(arg))
			if err != nil {
				return err
			}
			def.Wildcard = w
		default:
			return fmt.Errorf("unknown relation modifier %q", mod)
		}
	}
	p.schema.Relations[name] = def
	return nil
}

func parseWildcard(s string) (WildcardPos, error) {
	switch s {
	case "", "none":
		return WildcardNone, nil
	case "subject":
		return WildcardSubject, nil
	case "object":
		return WildcardObject, nil
	case "both":
		return WildcardBoth, nil
	default:
		return WildcardNone, fmt.Errorf("unknown wildcard position %q", s)
	}
}

func (p *parser) typeRelation(rest string) error {
	if p.current == nil {
		return fmt.Errorf("relation with targets outside a type block")
	}
	name, targets, _ := strings.Cut(rest, "->")
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("relation needs a name")
	}
	if _, dup := p.current.Relations[name]; dup {
		return fmt.Errorf("type %q declares relation %q twice", p.current.Name, name)
	}

	var out []TargetSpec
	for _, t := range strings.Split(targets, "|") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		typ, rel, _ := strings.Cut(t, "#")
		out = append(out, TargetSpec{Type: strings.TrimSpace(typ), Relation: strings.TrimSpace(rel)})
	}
	if len(out) == 0 {
		return fmt.Errorf("relation %q names no targets", name)
	}
	p.current.Relations[name] = out
	return nil
}

func (p *parser) rule(rest string) error {
	if p.current == nil {
		return fmt.Errorf("rule outside a type block")
	}
	action, body, ok := strings.Cut(rest, "=")
	action = strings.TrimSpace(action)
	if !ok || action == "" {
		return fmt.Errorf("rule needs an action and a body")
	}
	if _, dup := p.current.Rules[action]; dup {
		return fmt.Errorf("type %q declares rule %q twice", p.current.Name, action)
	}
	expr, err := parseExpr(body)
	if err != nil {
		return fmt.Errorf("rule %q: %s", action, err)
	}
	p.current.Rules[action] = expr
	return nil
}

// identTerm is a bare word whose meaning depends on the rest of the type: a
// relation it carries, or another of its rules. It is resolved once the whole
// type is parsed, so a rule may reference one declared further down.
type identTerm struct{ Name string }

func (identTerm) isExpr()          {}
func (t identTerm) String() string { return t.Name }

func parseExpr(body string) (Expr, error) {
	terms, ops, err := splitTerms(body)
	if err != nil {
		return nil, err
	}
	if len(terms) == 0 {
		return nil, fmt.Errorf("empty rule body")
	}

	cur, err := parseTerm(terms[0])
	if err != nil {
		return nil, err
	}
	for i, op := range ops {
		next, err := parseTerm(terms[i+1])
		if err != nil {
			return nil, err
		}
		switch op {
		case '+':
			if u, ok := cur.(Union); ok {
				u.Terms = append(u.Terms, next)
				cur = u
				continue
			}
			cur = Union{Terms: []Expr{cur, next}}
		case '&':
			if i, ok := cur.(Intersect); ok {
				i.Terms = append(i.Terms, next)
				cur = i
				continue
			}
			cur = Intersect{Terms: []Expr{cur, next}}
		default:
			cur = Exclude{Base: cur, Subtract: next}
		}
	}
	return cur, nil
}

// splitTerms separates a rule body on +, & and -, taking care that the - in an
// arrow belongs to the term rather than to the grammar. Operators associate to
// the left and there are no parentheses, so a mixed body reads strictly in
// order: a + b & c is (a + b) & c.
func splitTerms(body string) ([]string, []byte, error) {
	var (
		terms []string
		ops   []byte
		cur   strings.Builder
	)
	flush := func() error {
		t := strings.TrimSpace(cur.String())
		if t == "" {
			return fmt.Errorf("empty term")
		}
		terms = append(terms, t)
		cur.Reset()
		return nil
	}

	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c == '+' || c == '&':
			if err := flush(); err != nil {
				return nil, nil, err
			}
			ops = append(ops, c)
		case c == '-' && i+1 < len(body) && body[i+1] == '>':
			cur.WriteString("->")
			i++
		case c == '-':
			if err := flush(); err != nil {
				return nil, nil, err
			}
			ops = append(ops, '-')
		default:
			cur.WriteByte(c)
		}
	}
	if err := flush(); err != nil {
		return nil, nil, err
	}
	return terms, ops, nil
}

func parseTerm(t string) (Expr, error) {
	if rel, action, ok := strings.Cut(t, "->"); ok {
		rel = strings.TrimSpace(rel)
		action = strings.TrimSpace(action)
		if rel == "" || action == "" {
			return nil, fmt.Errorf("malformed arrow %q", t)
		}
		if strings.Contains(action, "->") {
			// A two-hop arrow would need an intermediate type to name, which
			// the rule on the far side already provides.
			return nil, fmt.Errorf("chained arrow %q: give the far type its own rule", t)
		}
		return ArrowTerm{Relation: rel, Target: action}, nil
	}
	return identTerm{Name: t}, nil
}

// resolve turns bare words into relation or rule references, now that every
// type is fully known.
func (p *parser) resolve() error {
	for _, t := range p.schema.Types {
		for action, expr := range t.Rules {
			resolved, err := resolveIdents(t, expr)
			if err != nil {
				return fmt.Errorf("%w: type %q rule %q: %s", ErrInvalidSchema, t.Name, action, err)
			}
			t.Rules[action] = resolved
		}
	}
	return nil
}

func resolveIdents(t *TypeDef, e Expr) (Expr, error) {
	switch x := e.(type) {
	case identTerm:
		if _, ok := t.Relations[x.Name]; ok {
			return RelationTerm{Relation: x.Name}, nil
		}
		if _, ok := t.Rules[x.Name]; ok {
			return RuleTerm{Action: x.Name}, nil
		}
		return nil, fmt.Errorf("%q is neither a relation nor a rule on this type", x.Name)
	case Union:
		out, err := resolveAll(t, x.Terms)
		if err != nil {
			return nil, err
		}
		return Union{Terms: out}, nil
	case Intersect:
		out, err := resolveAll(t, x.Terms)
		if err != nil {
			return nil, err
		}
		return Intersect{Terms: out}, nil
	case Exclude:
		base, err := resolveIdents(t, x.Base)
		if err != nil {
			return nil, err
		}
		sub, err := resolveIdents(t, x.Subtract)
		if err != nil {
			return nil, err
		}
		return Exclude{Base: base, Subtract: sub}, nil
	default:
		return e, nil
	}
}

func resolveAll(t *TypeDef, terms []Expr) ([]Expr, error) {
	out := make([]Expr, 0, len(terms))
	for _, term := range terms {
		r, err := resolveIdents(t, term)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
