package service_test

import (
	"context"
	"sort"
	"testing"

	"github.com/gsarmaonline/kyc/internal/accessmodel"
	"github.com/gsarmaonline/kyc/internal/service"
)

// The catalog in code only holds if it stays in step with the permissions the
// migrations seed. Code is authoritative; this fails when a migration adds a
// permission no gate can ask for, or removes one a gate names.
func TestCatalogMatchesSeededPermissions(t *testing.T) {
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
		if r.Key == accessmodel.CapOrganisationMember {
			t.Errorf("%s must not be a seeded permission: it is inherent to holding any role, not something a role hands out",
				accessmodel.CapOrganisationMember)
		}
	}
	sort.Strings(seeded)

	// The catalog is the seeded permissions plus the inherent capability.
	catalogued := []string{}
	for key := range accessmodel.Permissions {
		if key != accessmodel.CapOrganisationMember {
			catalogued = append(catalogued, key)
		}
	}
	sort.Strings(catalogued)

	if missing := difference(seeded, catalogued); len(missing) > 0 {
		t.Errorf("seeded but absent from the catalog: %v", missing)
	}
	if extra := difference(catalogued, seeded); len(extra) > 0 {
		t.Errorf("in the catalog but not seeded: %v", extra)
	}
}

// Every catalogued permission must resolve to a type and action the schema can
// actually answer, or a gate naming it would deny for the wrong reason.
func TestEveryCataloguedPermissionResolves(t *testing.T) {
	schema := accessmodel.MustLoad()
	for key, p := range accessmodel.Permissions {
		typ, ok := schema.Type(p.Type)
		if !ok {
			t.Errorf("%s: type %q is not declared", key, p.Type)
			continue
		}
		if _, ok := typ.Rules[p.Action]; !ok {
			t.Errorf("%s: type %q has no rule %q", key, p.Type, p.Action)
		}
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
