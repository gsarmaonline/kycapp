package mailer

import (
	"fmt"
	"strings"
)

// Config selects and configures a Mailer.
type Config struct {
	Provider string // resend|noop (default noop)
	APIKey   string // RESEND_API_KEY
	From     string // EMAIL_FROM e.g. "KYC <mail@example.com>"
}

// NewFromConfig returns the configured Mailer.
func NewFromConfig(cfg Config) (Mailer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "noop":
		return NewNoop(), nil
	case "resend":
		return NewResend(cfg.APIKey, cfg.From)
	default:
		return nil, fmt.Errorf("unknown EMAIL_PROVIDER %q (want resend|noop)", cfg.Provider)
	}
}
