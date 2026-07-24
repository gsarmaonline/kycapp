package automations

import "fmt"

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
		Description: "POST the event payload as JSON to a configured org webhook endpoint.",
		Params: []ActionParam{
			{Key: "webhook_id", Label: "Webhook", Required: true},
		},
		Requires: nil,
	}
}

func (callWebhookHandler) Requires() []string { return nil }

func (callWebhookHandler) Validate(params map[string]any) error {
	_, err := RequireStringParam(params, "webhook_id")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}
