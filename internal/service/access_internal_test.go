package service

import (
	"testing"

	"github.com/gsarmaonline/kyc/internal/authn"
)

// Break-glass is the one principal that does not go through the graph, and the
// set has to stay exactly one.
//
// A recovery credential used to match every clause: it is a service principal
// with no organisation and no key id, so it short-circuited every gate. Its
// edges on the star nodes were dead rows, Decision.Path never named it, and no
// test could see the difference, because a credential that reaches everything by
// short-circuit and one that reaches everything by edge answer every request
// identically. The distinction is only visible here.
func TestOnlyTheEnvironmentTokenIsBreakGlass(t *testing.T) {
	cases := []struct {
		name string
		p    authn.Principal
		want bool
	}{
		{
			name: "environment token",
			p:    authn.Principal{Kind: authn.KindService, Actor: "env-token"},
			want: true,
		},
		{
			name: "recovery credential",
			p:    authn.Principal{Kind: authn.KindService, RecoveryID: "rec_1", Actor: "recovery:incident"},
			want: false,
		},
		{
			name: "platform api key",
			p:    authn.Principal{Kind: authn.KindService, APIKeyID: "key_1", Actor: "api-key:ops"},
			want: false,
		},
		{
			name: "org scoped api key",
			p:    authn.Principal{Kind: authn.KindService, APIKeyID: "key_2", OrganisationID: "org_1"},
			want: false,
		},
		{
			name: "user session",
			p:    authn.Principal{Kind: authn.KindUser, UserID: "usr_1"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBreakGlass(tc.p); got != tc.want {
				t.Fatalf("isBreakGlass(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// Every principal that is not break-glass must resolve to a node, or it cannot
// be walked and the gate would have to invent an answer for it.
func TestEveryNonBreakGlassPrincipalHasANode(t *testing.T) {
	cases := map[string]struct {
		p    authn.Principal
		want string
	}{
		"recovery": {authn.Principal{Kind: authn.KindService, RecoveryID: "rec_1"}, "recovery:rec_1"},
		"key":      {authn.Principal{Kind: authn.KindService, APIKeyID: "key_1"}, "key:key_1"},
		"user":     {authn.Principal{Kind: authn.KindUser, UserID: "usr_1"}, "user:usr_1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			node, err := principalNode(tc.p)
			if err != nil {
				t.Fatalf("principalNode: %v", err)
			}
			if node.String() != tc.want {
				t.Fatalf("principalNode = %s, want %s", node, tc.want)
			}
		})
	}
}
