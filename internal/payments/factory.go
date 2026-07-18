package payments

import (
	"fmt"
	"strings"
)

// Config selects and configures a Processor.
type Config struct {
	Provider      string // stripe|noop (default noop)
	StripeSecret  string
	WebhookSecret string
}

// NewFromConfig returns the configured Processor.
func NewFromConfig(cfg Config) (Processor, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "noop":
		return NewNoop(), nil
	case "stripe":
		return NewStripe(cfg.StripeSecret, cfg.WebhookSecret)
	default:
		return nil, fmt.Errorf("unknown PAYMENTS_PROVIDER %q (want stripe|noop)", cfg.Provider)
	}
}
