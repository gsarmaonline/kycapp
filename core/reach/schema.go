package reach

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidSchema means the schema will not validate. Every schema is checked
// before it runs, so a typo fails at load rather than as a silent denial in
// production.
var ErrInvalidSchema = errors.New("reach: invalid schema")

// WildcardPos says where a relation accepts a star node.
//
// The declaration is not decoration. Without it, document:* #parent
// folder:public would reparent every document in a tenant with one write.
// Structural relations take WildcardNone; relations that confer access take it
// on whichever side makes sense.
type WildcardPos uint8

const (
	// WildcardNone forbids a star node at either end.
	WildcardNone WildcardPos = iota
	// WildcardSubject allows document:d1 #viewer user:* - every user.
	WildcardSubject
	// WildcardObject allows document:* #viewer group:audit - every document.
	WildcardObject
	// WildcardBoth allows either end.
	WildcardBoth
)

func (w WildcardPos) allowsSubject() bool { return w == WildcardSubject || w == WildcardBoth }
func (w WildcardPos) allowsObject() bool  { return w == WildcardObject || w == WildcardBoth }

func (w WildcardPos) String() string {
	switch w {
	case WildcardSubject:
		return "subject"
	case WildcardObject:
		return "object"
	case WildcardBoth:
		return "both"
	default:
		return "none"
	}
}

// RelationDef declares an edge kind for the whole namespace.
type RelationDef struct {
	Name string
	// Transitive makes the walk follow this relation to closure. This single
	// flag is the whole grouping mechanism: a group inside a group, a folder
	// inside a folder and an action that covers another are all the same
	// operation.
	Transitive bool
	// Identity is the one kind the walk follows outward from the *subject*, so
	// a key carries the reach of whoever it acts as. Everything else is
	// followed inward from the resource.
	Identity bool
	Wildcard WildcardPos
}

// TargetSpec is one legal far end of a relation: a type, optionally with the
// relation to follow at it.
type TargetSpec struct {
	Type string
	// Relation is empty for a plain node target, e.g. "user". Otherwise it is
	// a userset target, e.g. group#member.
	Relation string
}

func (t TargetSpec) String() string {
	if t.Relation == "" {
		return t.Type
	}
	return t.Type + "#" + t.Relation
}

// TypeDef is one resource type: which relations it carries, and how each action
// resolves on it.
type TypeDef struct {
	Name string
	// Relations maps a relation name to its legal targets.
	Relations map[string][]TargetSpec
	// Rules maps an action to the expression that answers it.
	Rules map[string]Expr
}

// Schema is one namespace's complete declaration. It is closed on purpose: a
// grant naming an undeclared action or relation reaches nothing.
type Schema struct {
	Namespace string
	Actions   []string
	Relations map[string]RelationDef
	Types     map[string]*TypeDef
}

// NewSchema returns an empty schema for a namespace.
func NewSchema(namespace string) *Schema {
	return &Schema{
		Namespace: namespace,
		Relations: map[string]RelationDef{},
		Types:     map[string]*TypeDef{},
	}
}

// Relation returns a relation declaration.
func (s *Schema) Relation(name string) (RelationDef, bool) {
	d, ok := s.Relations[name]
	return d, ok
}

// Type returns a type declaration.
func (s *Schema) Type(name string) (*TypeDef, bool) {
	t, ok := s.Types[name]
	return t, ok
}

// HasAction reports whether the action is in the namespace vocabulary.
func (s *Schema) HasAction(name string) bool {
	for _, a := range s.Actions {
		if a == name {
			return true
		}
	}
	return false
}

// identityRelations returns the relations the walk follows outward from a
// subject, sorted so behaviour does not depend on map order.
func (s *Schema) identityRelations() []string {
	var out []string
	for name, d := range s.Relations {
		if d.Identity {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// --- Expressions ---

// Expr is a rule body. The grammar is closed: union, traversal, subtraction,
// and a reference to another rule on the same type. Nothing else.
type Expr interface {
	isExpr()
	String() string
}

// RelationTerm names a relation on this type: viewer.
type RelationTerm struct{ Relation string }

// RuleTerm references another rule on the same type: read = viewer + write.
type RuleTerm struct{ Action string }

// ArrowTerm walks a relation and asks again at the far end: parent->write.
//
// Target is a rule on the far type when one exists, and otherwise a relation on
// it. Both are wanted: parent->write asks a permission of the container, while
// classified->withheld asks who is named on a marker node that carries no rules
// of its own.
type ArrowTerm struct {
	Relation string
	Target   string
}

// Union allows any of its terms. Order is irrelevant, because reachability only
// ever adds.
type Union struct{ Terms []Expr }

// Exclude subtracts. It lives in a rule, never as a free-floating deny edge: a
// set expression evaluates at one point, so no priority field and no conflict
// resolution appear anywhere.
type Exclude struct {
	Base     Expr
	Subtract Expr
}

func (RelationTerm) isExpr() {}
func (RuleTerm) isExpr()     {}
func (ArrowTerm) isExpr()    {}
func (Union) isExpr()        {}
func (Exclude) isExpr()      {}

func (e RelationTerm) String() string { return e.Relation }
func (e RuleTerm) String() string     { return e.Action }
func (e ArrowTerm) String() string    { return e.Relation + "->" + e.Target }

func (e Union) String() string {
	parts := make([]string, 0, len(e.Terms))
	for _, t := range e.Terms {
		parts = append(parts, t.String())
	}
	return strings.Join(parts, " + ")
}

func (e Exclude) String() string { return e.Base.String() + " - " + e.Subtract.String() }

// --- Validation ---

// Validate checks the schema is internally consistent. It is the step that
// makes invariant "a typo cannot reach a grant" hold, so it runs at load time
// and refuses to boot rather than denying quietly later.
func (s *Schema) Validate() error {
	if strings.TrimSpace(s.Namespace) == "" {
		return fmt.Errorf("%w: empty namespace", ErrInvalidSchema)
	}

	seenAction := map[string]struct{}{}
	for _, a := range s.Actions {
		if err := validIdent(a); err != nil {
			return fmt.Errorf("%w: action %q: %s", ErrInvalidSchema, a, err)
		}
		if _, dup := seenAction[a]; dup {
			return fmt.Errorf("%w: duplicate action %q", ErrInvalidSchema, a)
		}
		seenAction[a] = struct{}{}
	}

	for name, d := range s.Relations {
		if err := validIdent(name); err != nil {
			return fmt.Errorf("%w: relation %q: %s", ErrInvalidSchema, name, err)
		}
		if d.Name != name {
			return fmt.Errorf("%w: relation %q is keyed as %q", ErrInvalidSchema, d.Name, name)
		}
		if d.Identity && d.Wildcard != WildcardNone {
			// An identity edge says "this principal is also that one". A star
			// on it would make every principal of a type the same principal.
			return fmt.Errorf("%w: identity relation %q may not accept a wildcard", ErrInvalidSchema, name)
		}
	}

	names := make([]string, 0, len(s.Types))
	for name := range s.Types {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		t := s.Types[name]
		if err := validIdent(name); err != nil {
			return fmt.Errorf("%w: type %q: %s", ErrInvalidSchema, name, err)
		}
		if t.Name != name {
			return fmt.Errorf("%w: type %q is keyed as %q", ErrInvalidSchema, t.Name, name)
		}
		if err := s.validateTypeRelations(t); err != nil {
			return err
		}
		if err := s.validateTypeRules(t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Schema) validateTypeRelations(t *TypeDef) error {
	rels := make([]string, 0, len(t.Relations))
	for r := range t.Relations {
		rels = append(rels, r)
	}
	sort.Strings(rels)

	for _, r := range rels {
		if _, ok := s.Relations[r]; !ok {
			return fmt.Errorf("%w: type %q uses undeclared relation %q", ErrInvalidSchema, t.Name, r)
		}
		targets := t.Relations[r]
		if len(targets) == 0 {
			return fmt.Errorf("%w: type %q relation %q names no targets", ErrInvalidSchema, t.Name, r)
		}
		for _, target := range targets {
			if _, ok := s.Types[target.Type]; !ok {
				return fmt.Errorf("%w: type %q relation %q targets undeclared type %q",
					ErrInvalidSchema, t.Name, r, target.Type)
			}
			if target.Relation == "" {
				continue
			}
			if _, ok := s.Relations[target.Relation]; !ok {
				return fmt.Errorf("%w: type %q relation %q targets undeclared relation %q",
					ErrInvalidSchema, t.Name, r, target.Relation)
			}
			if _, ok := s.Types[target.Type].Relations[target.Relation]; !ok {
				return fmt.Errorf("%w: type %q relation %q targets %s, but %q carries no %q",
					ErrInvalidSchema, t.Name, r, target, target.Type, target.Relation)
			}
		}
	}
	return nil
}

func (s *Schema) validateTypeRules(t *TypeDef) error {
	actions := make([]string, 0, len(t.Rules))
	for a := range t.Rules {
		actions = append(actions, a)
	}
	sort.Strings(actions)

	for _, a := range actions {
		if !s.HasAction(a) {
			return fmt.Errorf("%w: type %q declares rule for undeclared action %q", ErrInvalidSchema, t.Name, a)
		}
		if err := s.validateExpr(t, t.Rules[a]); err != nil {
			return fmt.Errorf("%w: type %q rule %q: %s", ErrInvalidSchema, t.Name, a, err)
		}
	}
	// A rule that references itself, directly or through a chain, would make
	// the walk depend on a cycle in the schema rather than in the data. The
	// visited set would terminate it, but the rule would silently mean nothing.
	for _, a := range actions {
		if err := s.checkRuleCycle(t, a, a, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Schema) validateExpr(t *TypeDef, e Expr) error {
	switch x := e.(type) {
	case RelationTerm:
		if _, ok := t.Relations[x.Relation]; !ok {
			return fmt.Errorf("relation %q is not carried by this type", x.Relation)
		}
	case RuleTerm:
		if _, ok := t.Rules[x.Action]; !ok {
			return fmt.Errorf("no rule %q on this type", x.Action)
		}
	case ArrowTerm:
		targets, ok := t.Relations[x.Relation]
		if !ok {
			return fmt.Errorf("relation %q is not carried by this type", x.Relation)
		}
		for _, target := range targets {
			tt := s.Types[target.Type]
			if tt == nil {
				continue
			}
			if _, ok := tt.Rules[x.Target]; ok {
				continue
			}
			if _, ok := tt.Relations[x.Target]; ok {
				continue
			}
			return fmt.Errorf("%s: type %q has no rule or relation %q", x, target.Type, x.Target)
		}
	case Union:
		if len(x.Terms) == 0 {
			return errors.New("empty union")
		}
		for _, term := range x.Terms {
			if err := s.validateExpr(t, term); err != nil {
				return err
			}
		}
	case Exclude:
		if err := s.validateExpr(t, x.Base); err != nil {
			return err
		}
		return s.validateExpr(t, x.Subtract)
	default:
		return fmt.Errorf("unknown expression %T", e)
	}
	return nil
}

func (s *Schema) checkRuleCycle(t *TypeDef, start, current string, seen map[string]bool) error {
	if seen[current] {
		return fmt.Errorf("%w: type %q rule %q references itself", ErrInvalidSchema, t.Name, start)
	}
	seen[current] = true
	defer delete(seen, current)

	var walk func(Expr) error
	walk = func(e Expr) error {
		switch x := e.(type) {
		case RuleTerm:
			return s.checkRuleCycle(t, start, x.Action, seen)
		case Union:
			for _, term := range x.Terms {
				if err := walk(term); err != nil {
					return err
				}
			}
		case Exclude:
			if err := walk(x.Base); err != nil {
				return err
			}
			return walk(x.Subtract)
		}
		return nil
	}
	return walk(t.Rules[current])
}

func validIdent(s string) error {
	if s == "" {
		return errors.New("empty")
	}
	if s != strings.TrimSpace(s) {
		return errors.New("has surrounding whitespace")
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return errors.New("must be lowercase letters, digits and underscores, starting with a letter")
		}
	}
	return nil
}
