package automations

import "github.com/gsarmaonline/kyc/core/resources"

func init() {
	RegisterActionHandler(sendEmailHandler{})
}

type sendEmailHandler struct{}

func (sendEmailHandler) Type() string { return ActionSendEmail }

func (sendEmailHandler) Info() ActionInfo {
	return ActionInfo{
		Type:        ActionSendEmail,
		Label:       "Send email",
		Description: "Render an org email template and deliver it to the app user's email.",
		Params: []ActionParam{
			{Key: "template_key", Label: "Template key", Required: true},
		},
		Requires: []string{resources.SubjectAppUser},
	}
}

func (sendEmailHandler) Requires() []string {
	return []string{resources.SubjectAppUser}
}

func (sendEmailHandler) Validate(params map[string]any) error {
	_, err := RequireStringParam(params, "template_key")
	return err
}
