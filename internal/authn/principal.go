package authn

import "context"

// Kind identifies how the caller authenticated.
type Kind string

const (
	KindUser    Kind = "user"
	KindService Kind = "service"
)

// Principal is the authenticated caller attached to a request context.
type Principal struct {
	Kind           Kind
	UserID         string
	OrganisationID string // set for org-scoped service API keys
	APIKeyID       string
	// RecoveryID is set for a recovery credential. It reaches every
	// organisation, but as a grant the evaluator weighs, not as a bypass.
	RecoveryID string
	// OwnerUserID is the user an API key belongs to. A key's capabilities are
	// the intersection of this user's grants and the key's scopes, so a key can
	// never exceed its owner.
	OwnerUserID   string
	Scopes        []string // org API key permission scopes; empty = full org access
	PlatformAdmin bool
	SessionID     string
	Actor         string // audit label
}

type ctxKey int

const principalKey ctxKey = 1

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// IsPlatform is gone.
//
// It was the coarse staff bypass that RequirePlatform was deleted to remove,
// living on as a method. Six handlers used it to skip the capability check
// entirely, so any member of the platform organisation — a read-only support
// role included — could edit any user's status, and any API key with no
// organisation counted as staff whatever its owner held. It also disagreed with
// isBreakGlass, which deliberately refuses a stored key.
//
// Ask the graph instead. Service.RequirePlatformCapability for a named
// capability at global scope, and Service.ReachesEveryOrganisation for the
// listing question. Both go through the same walk as everything else, so a
// read-only role stays read-only.
//
// PlatformAdmin survives as a field: /v1/me reports it so the UI can show staff
// screens. It is a display fact, never a gate.

// ActorLabel is a stable string for audit logs.
func (p Principal) ActorLabel() string {
	if p.Actor != "" {
		return p.Actor
	}
	if p.Kind == KindUser && p.UserID != "" {
		return "user:" + p.UserID
	}
	return "anonymous"
}
