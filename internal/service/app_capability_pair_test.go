package service_test

import (
	"net/http"
	"testing"
)

// A capability is a pair, not a string.
//
// It was stored as one flat key, 'document:read', and MerchantSchema cut it on
// ':' to rebuild the grid it needed. The grid was the model; the string was the
// lossy form, and the lossiness cost three things: a malformed key failed one
// page away from where it was written, the reserved-name check ran per key
// rather than once per resource, and nothing could ask which actions exist
// without splitting strings in SQL.
//
// key survives as a generated column because it is the merchant-facing name and
// what roles carry in their capability lists. Deriving it means the name and the
// pair cannot drift apart.

// The halves round-trip. A merchant still writes and reads resource:action, so
// nothing about the vocabulary they authored has moved.
func TestCapabilityKeyRoundTripsThroughThePair(t *testing.T) {
	f := newAppAccess(t, "cappair")
	f.declareCapability(t, "document:read")

	code, out := f.get(t, "/app-capabilities")
	if code != http.StatusOK {
		t.Fatalf("list capabilities: %d %v", code, out)
	}
	items := out["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("want one capability, got %d", len(items))
	}
	if key := items[0].(map[string]any)["key"]; key != "document:read" {
		t.Fatalf("key = %v, wanted document:read", key)
	}
}

// The pair is what makes a role's capability list resolvable, so a role
// carrying the key still reaches through a grant exactly as before. This is the
// end-to-end check that the storage change is invisible to the model.
func TestCapabilityPairStillReachesThroughAGrant(t *testing.T) {
	f := newAppAccess(t, "cappairreach")
	f.declareScope(t, "project")
	f.declareCapability(t, "document:read")
	roleID := f.createRole(t, "reader", []string{"document:read"}, nil)
	userID := f.createAppUser(t, "ana@customer.com")

	if code, out := f.post(t, "/app-grants", map[string]any{
		"app_user_id": userID, "role_id": roleID,
		"scope_kind": "project", "scope_id": "apollo",
	}); code != http.StatusCreated {
		t.Fatalf("grant: %d %v", code, out)
	}

	set := f.accessFor(t, userID)
	grants := set["grants"].([]any)
	if len(grants) != 1 {
		t.Fatalf("want one grant, got %d", len(grants))
	}
	caps := grants[0].(map[string]any)["capabilities"].([]any)
	if len(caps) != 1 || caps[0] != "document:read" {
		t.Fatalf("capabilities = %v, wanted [document:read]", caps)
	}
}

// A key with no action, or no resource, is refused where it is written. It used
// to pass this check and fail later in MerchantSchema, which is a different
// request on a different page, so the merchant saw the mistake nowhere near the
// form that made it.
func TestAMalformedCapabilityIsRefusedAtWriteTime(t *testing.T) {
	f := newAppAccess(t, "capmalformed")
	for _, bad := range []string{"document", "document:", ":read", "", "a:b:c"} {
		code, _ := f.post(t, "/app-capabilities", map[string]any{"key": bad})
		if code != http.StatusBadRequest {
			t.Errorf("capability %q must be refused at write time, got %d", bad, code)
		}
	}
}

// Declaring the same pair twice is a conflict, whichever way it is spelled. The
// uniqueness moved from the key to the pair, and they must not disagree.
func TestTheSamePairCannotBeDeclaredTwice(t *testing.T) {
	f := newAppAccess(t, "capdup")
	f.declareCapability(t, "document:read")

	code, _ := f.post(t, "/app-capabilities", map[string]any{"key": "document:read"})
	if code != http.StatusConflict {
		t.Fatalf("re-declaring a capability must conflict, got %d", code)
	}

	// A different action on the same resource is a different cell, not a clash.
	if code, out := f.post(t, "/app-capabilities", map[string]any{"key": "document:write"}); code != http.StatusCreated {
		t.Fatalf("another action on the same resource must be allowed: %d %v", code, out)
	}
	// And the same action on a different resource likewise.
	if code, out := f.post(t, "/app-capabilities", map[string]any{"key": "invoice:read"}); code != http.StatusCreated {
		t.Fatalf("the same action on another resource must be allowed: %d %v", code, out)
	}
}
