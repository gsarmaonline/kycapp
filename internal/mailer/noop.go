package mailer

import (
	"context"
	"log/slog"
	"strings"
)

// Noop logs messages and does not deliver.
type Noop struct{}

func NewNoop() *Noop { return &Noop{} }

func (n *Noop) Name() string { return "noop" }

func (n *Noop) Send(ctx context.Context, msg Message) (string, error) {
	_ = ctx
	slog.Info("mailer noop send",
		"to", strings.Join(msg.To, ","),
		"subject", msg.Subject,
		"html_len", len(msg.HTML),
		"text_len", len(msg.Text),
	)
	return "noop", nil
}
