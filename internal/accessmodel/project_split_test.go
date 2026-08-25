package accessmodel

import (
	"strings"
	"testing"
)

// A semicolon in a comment used to cut a statement in half.
//
// The projection files are split on the semicolon so each statement can carry
// the same parameter. Splitting the raw text meant a `--` comment containing a
// semicolon produced two fragments, and Postgres reported a syntax error on
// whatever word followed it. Both projection files carried a warning against
// writing one. The warning was violated twice, once by the person who wrote it,
// which is roughly what a warning is worth against ordinary prose.
func TestASemicolonInACommentDoesNotSplitAStatement(t *testing.T) {
	src := `
-- A comment; with a semicolon in it, which is where prose puts them.
INSERT INTO t (a) VALUES ($1);

-- Another; and the statement after it must survive.
INSERT INTO u (b) VALUES ($1);
`
	got := splitStatements(src)
	if len(got) != 2 {
		t.Fatalf("want 2 statements, got %d: %#v", len(got), got)
	}
	for i, stmt := range got {
		if !strings.Contains(stmt, "INSERT INTO") {
			t.Errorf("statement %d lost its verb: %q", i+1, stmt)
		}
	}
}

// The real files are the thing that has to keep working. Every statement in
// both projections is an insert into the edge table, so anything else means a
// statement was cut somewhere it should not have been.
func TestBothProjectionsSplitIntoRunnableStatements(t *testing.T) {
	for name, src := range map[string]string{
		"kyc":      projectionSQL,
		"merchant": merchantProjectionSQL,
	} {
		stmts := splitStatements(src)
		if len(stmts) == 0 {
			t.Fatalf("%s projection produced no statements", name)
		}
		for i, stmt := range stmts {
			// Contains() is not enough, and that is the whole lesson. A statement
			// cut by a semicolon in the header comment still *contains* the
			// INSERT further down; what it also carries is the tail of a
			// sentence in front of it, which is what Postgres choked on.
			// So: once comments are gone, the statement must *begin* with the
			// verb and nothing else.
			body := strings.TrimSpace(stripSQLComments(stmt))
			if !strings.HasPrefix(body, "INSERT INTO reach_edges") {
				head := body
				if len(head) > 80 {
					head = head[:80]
				}
				t.Errorf("%s projection statement %d has something before its verb: %q", name, i+1, head)
			}
		}
	}
}
