package automations

import (
	"fmt"
	"net/url"
	"strings"
)

func init() {
	RegisterActionHandler(callWebhookHandler{})
}

const ActionCallWebhook = "call_webhook"

type callWebhookHandler struct{}

func (callWebhookHandler) Type() string { return ActionCallWebhook }

func (callWebhookHandler) Info() ActionInfo {
	return ActionInfo{
		Type:        ActionCallWebhook,
		Label:       "Call webhook",
		Description: "POST the event payload as JSON to an HTTPS URL. Optional secret is sent as X-KYC-Webhook-Secret.",
		Params: []ActionParam{
			{Key: "url", Label: "URL", Required: true},
			{Key: "secret", Label: "Shared secret (optional)", Required: false},
		},
		Requires: nil,
	}
}

func (callWebhookHandler) Requires() []string { return nil }

func (callWebhookHandler) Validate(params map[string]any) error {
	raw, err := RequireStringParam(params, "url")
	if err != nil {
		return err
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("url is invalid")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return fmt.Errorf("url must be http or https")
	}
	return nil
}
