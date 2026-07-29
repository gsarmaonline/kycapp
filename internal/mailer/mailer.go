package mailer

import "context"

// Message is a transactional email ready to send.
type Message struct {
	To      []string
	Subject string
	HTML    string
	Text    string
	From    string // optional; provider default used when empty
	ReplyTo string
	Tags    map[string]string
}

// Mailer delivers transactional email.
type Mailer interface {
	Name() string
	Send(ctx context.Context, msg Message) (providerRef string, err error)
}
