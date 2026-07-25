package mailer

import (
	"context"
	"sync"
)

// Recording is an in-memory Mailer for local/e2e tests (no network).
type Recording struct {
	mu   sync.Mutex
	Msgs []Message
}

func NewRecording() *Recording { return &Recording{} }

func (r *Recording) Name() string { return "recording" }

func (r *Recording) Send(ctx context.Context, msg Message) (string, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Msgs = append(r.Msgs, msg)
	return "recording", nil
}

// Messages returns a copy of recorded sends.
func (r *Recording) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Message, len(r.Msgs))
	copy(out, r.Msgs)
	return out
}

// Reset clears recorded sends.
func (r *Recording) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Msgs = nil
}
