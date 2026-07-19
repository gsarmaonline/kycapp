package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultResendAPIURL = "https://api.resend.com/emails"

// Resend delivers via the Resend HTTP API.
type Resend struct {
	apiKey  string
	from    string
	apiURL  string
	client  *http.Client
}

func NewResend(apiKey, from string) (*Resend, error) {
	apiKey = strings.TrimSpace(apiKey)
	from = strings.TrimSpace(from)
	if apiKey == "" {
		return nil, fmt.Errorf("RESEND_API_KEY is required when EMAIL_PROVIDER=resend")
	}
	if from == "" {
		return nil, fmt.Errorf("EMAIL_FROM is required when EMAIL_PROVIDER=resend")
	}
	return &Resend{
		apiKey: apiKey,
		from:   from,
		apiURL: defaultResendAPIURL,
		client: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (r *Resend) Name() string { return "resend" }

type resendRequest struct {
	From    string      `json:"from"`
	To      []string    `json:"to"`
	Subject string      `json:"subject"`
	HTML    string      `json:"html,omitempty"`
	Text    string      `json:"text,omitempty"`
	ReplyTo []string    `json:"reply_to,omitempty"`
	Tags    []resendTag `json:"tags,omitempty"`
}

type resendTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type resendResponse struct {
	ID string `json:"id"`
}

func (r *Resend) Send(ctx context.Context, msg Message) (string, error) {
	to := cleanAddrs(msg.To)
	if len(to) == 0 {
		return "", fmt.Errorf("at least one recipient is required")
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return "", fmt.Errorf("subject is required")
	}
	if strings.TrimSpace(msg.HTML) == "" && strings.TrimSpace(msg.Text) == "" {
		return "", fmt.Errorf("html or text body is required")
	}

	body := resendRequest{
		From:    r.from,
		To:      to,
		Subject: strings.TrimSpace(msg.Subject),
		HTML:    msg.HTML,
		Text:    msg.Text,
	}
	if rt := strings.TrimSpace(msg.ReplyTo); rt != "" {
		body.ReplyTo = []string{rt}
	}
	for k, v := range msg.Tags {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "" || v == "" {
			continue
		}
		body.Tags = append(body.Tags, resendTag{Name: k, Value: v})
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.apiURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("resend: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var out resendResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("resend: decode response: %w", err)
	}
	if out.ID == "" {
		return "", fmt.Errorf("resend: empty id in response")
	}
	return out.ID, nil
}

func cleanAddrs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, a := range in {
		a = strings.TrimSpace(a)
		if a != "" {
			out = append(out, a)
		}
	}
	return out
}
