package service_test

import (
	"context"
	"sort"
	"testing"

	"github.com/gsarmaonline/kyc/internal/service"
)

// Invariant 3 only holds if the code-defined registry stays in step with the
// permissions the migrations seed. Code is authoritative; this fails when a
// migration adds a permission no gate can ask for, or removes one a gate names.
func TestCapabilityRegistryMatchesSeededPermissions(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t, ctx)
	svc := service.New(db)

	rows, err := svc.ListPermissions(ctx, "", "")
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	seeded := make([]string, 0, len(rows))
	for _, r := range rows {
		seeded = append(seeded, r.Key)
		if r.Key == service.CapOrganisationMember {
			t.Errorf("%s must not be a seeded permission: it is inherent to holding any grant, not something a role hands out", service.CapOrganisationMember)
		}
	}
	sort.Strings(seeded)

	// The registry is the seeded permissions plus the inherent capability.
	registered := []string{}
	for _, k := range service.KYCCapabilities.Keys() {
		if k != service.CapOrganisationMember {
			registered = append(registered, k)
		}
	}
	sort.Strings(registered)

	if missing := difference(seeded, registered); len(missing) > 0 {
		t.Errorf("seeded but not registered in code: %v", missing)
	}
	if extra := difference(registered, seeded); len(extra) > 0 {
		t.Errorf("registered in code but not seeded: %v", extra)
	}
}

func difference(a, b []string) []string {
	in := make(map[string]struct{}, len(b))
	for _, s := range b {
		in[s] = struct{}{}
	}
	var out []string
	for _, s := range a {
		if _, ok := in[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}
