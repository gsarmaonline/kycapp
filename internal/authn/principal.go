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
	Kind          Kind
	UserID        string
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

// IsPlatform returns true for service tokens and platform-admin users.
func (p Principal) IsPlatform() bool {
	return p.Kind == KindService || p.PlatformAdmin
}

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
