// Package accessmodel expresses KYC's own authorisation domain in the reach
// schema language.
//
// This is step one of the migration in docs/access-by-reachability.md: state
// the current domain in the new model, and find out where it does not fit
// before any storage or any gate depends on the answer. Nothing here is wired
// to a request path.
//
// # How the current model projects onto this one
//
//	permissions row          a (type, action) pair
//	roles row                a role node
//	role_permissions row     an edge: <area>:<org> #can_<action> role:<id>#holder
//	memberships row          an edge: role:<id> #holder user:<id>, with expiry
//	platform org membership  the same edges, written on the <area>:* star node
//	api_keys row             an edge: key:<id> #actor user:<owner>
//
// Every area type is keyed by the organisation id, so api_keys:acme is "the API
// keys of acme". That keeps the resource noun in the type where it can be
// checked, rather than in an opaque compound id, and it leaves room for
// instance-level types to arrive later without moving anything.
package accessmodel

import (
	"fmt"

	"github.com/gsarmaonline/kyc/core/reach"
)

// Schema is KYC's own namespace.
//
// Two things are worth reading closely.
//
// Global reach is not a flag and not a role name. It is an edge written on a
// star node: api_keys:* #can_manage role:platform_admin#holder. So staff reach
// is visible in the data, a read-only support role stays read-only, and the
// delegation rule already refuses anyone who does not hold the star node from
// issuing one.
//
// Organisation lifecycle is a subtraction, and the ordering is deliberate.
// `belongs - suspended + oversees` reads left to right: a member of a suspended
// tenant loses reach, and a platform principal holding `oversees` does not,
// because their term sits outside the subtraction. That is what lets a
// suspended tenant still be seen and restored.
const Schema = `
namespace kyc

action member, read, write, update, manage, invite, remove

// --- principals ---
relation member_of : transitive
relation holder    : direct
relation actor     : identity

// --- structure ---
relation org       : direct
relation belongs   : direct, wildcard object
relation suspended : direct
relation oversees  : direct, wildcard object

// --- what a role grants, one relation per action ---
relation can_read   : direct, wildcard object
relation can_write  : direct, wildcard object
relation can_update : direct, wildcard object
relation can_manage : direct, wildcard object
relation can_invite : direct, wildcard object
relation can_remove : direct, wildcard object

type user

type group
  relation member_of -> user | group#member_of

// A key holds nothing of its own. It reaches exactly what its actor reaches,
// so demoting the owner demotes the key on the next request and deleting the
// edge stops it.
type key
  relation actor -> user | role#holder

// A role is a named set of holders. Membership expiry lives on the holder edge,
// which is what makes time-boxed access the cheap option rather than the
// diligent one.
type role
  relation holder -> user | group#member_of

type organisation
  relation belongs    -> user | role#holder
  relation suspended  -> user | role#holder
  relation oversees   -> user | role#holder
  relation can_read   -> user | role#holder
  relation can_update -> user | role#holder
  rule member = belongs - suspended + oversees
  rule read   = can_read
  rule update = can_update

type members
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_invite -> user | role#holder
  relation can_remove -> user | role#holder
  rule read   = can_read
  rule invite = can_invite
  rule remove = can_remove

type org_roles
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type api_keys
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type app_users
  relation org       -> organisation
  relation can_read  -> user | role#holder
  relation can_write -> user | role#holder
  rule read  = can_read
  rule write = can_write

type app_access
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type attributes
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type automations
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type billing
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type email_templates
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type product_features
  relation org        -> organisation
  relation can_read   -> user | role#holder
  relation can_manage -> user | role#holder
  rule read   = can_read
  rule manage = can_manage

type activity
  relation org      -> organisation
  relation can_read -> user | role#holder
  rule read = can_read

type usage
  relation org      -> organisation
  relation can_read -> user | role#holder
  rule read = can_read
`

// PermissionKey maps a permission key from the current catalog to the (type,
// action) pair that replaces it.
//
// This table is the migration contract. TestEveryPermissionKeyMaps checks it
// against the registry the current system boots from, so a key added on either
// side without the other fails the build rather than silently losing a gate.
type PermissionKey struct {
	Type   string
	Action string
}

// Permissions is the whole current catalog, projected.
var Permissions = map[string]PermissionKey{
	"organisation:member": {"organisation", "member"},
	"organisation:read":   {"organisation", "read"},
	"organisation:update": {"organisation", "update"},

	"members:read":   {"members", "read"},
	"members:invite": {"members", "invite"},
	"members:remove": {"members", "remove"},

	"roles:read":   {"org_roles", "read"},
	"roles:manage": {"org_roles", "manage"},

	"api_keys:read":   {"api_keys", "read"},
	"api_keys:manage": {"api_keys", "manage"},

	"app_users:read":  {"app_users", "read"},
	"app_users:write": {"app_users", "write"},

	"app_access:read":   {"app_access", "read"},
	"app_access:manage": {"app_access", "manage"},

	"attributes:read":   {"attributes", "read"},
	"attributes:manage": {"attributes", "manage"},

	"automations:read":   {"automations", "read"},
	"automations:manage": {"automations", "manage"},

	"billing:read":   {"billing", "read"},
	"billing:manage": {"billing", "manage"},

	"email_templates:read":   {"email_templates", "read"},
	"email_templates:manage": {"email_templates", "manage"},

	"product_features:read":   {"product_features", "read"},
	"product_features:manage": {"product_features", "manage"},

	"activity:read": {"activity", "read"},
	"usage:read":    {"usage", "read"},
}

// GrantRelation is the relation a role_permissions row becomes. One relation
// per action keeps the current model's granularity: a role holding
// members:invite and not members:remove stays that way.
func GrantRelation(action string) string { return "can_" + action }

// Load parses and validates the schema. It fails loudly rather than returning a
// partially usable one, and it refuses a schema carrying inert declarations,
// because a schema that says something the engine ignores is worse than one
// that says nothing.
func Load() (*reach.Schema, error) {
	s, err := reach.Parse(Schema)
	if err != nil {
		return nil, err
	}
	if w := s.Warnings(); len(w) > 0 {
		return nil, fmt.Errorf("accessmodel: schema has inert declarations: %v", w)
	}
	return s, nil
}

// MustLoad is Load for callers that cannot proceed without the schema.
func MustLoad() *reach.Schema {
	s, err := Load()
	if err != nil {
		panic(err)
	}
	return s
}

// Area returns the node for one organisation's slice of a resource type, e.g.
// api_keys:acme.
func Area(typeName, orgID string) reach.NodeRef { return reach.Node(typeName, orgID) }

// EveryArea returns the star node for a resource type: every organisation's
// slice of it, present and future. This is what global reach is written on.
func EveryArea(typeName string) reach.NodeRef { return reach.Star(typeName) }
