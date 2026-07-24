package automations

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeActionsChainsLinear(t *testing.T) {
	out := NormalizeActions([]Action{
		{Type: ActionSendEmail, Params: map[string]any{"template_key": "a"}},
		{Type: ActionSendEmail, Params: map[string]any{"template_key": "b"}},
	})
	if len(out) != 2 || out[0].ID != "a1" || out[1].ID != "a2" {
		t.Fatalf("ids=%v %v", out[0].ID, out[1].ID)
	}
	if out[0].OnSuccess != "a2" || out[1].OnSuccess != "" {
		t.Fatalf("chain=%q → %q", out[0].OnSuccess, out[1].OnSuccess)
	}
}

func TestRunActionGraphOnErrorBranch(t *testing.T) {
	actions := []Action{
		{ID: "insert", Type: ActionDBInsert, OnSuccess: "email", OnError: "hook"},
		{ID: "email", Type: ActionSendEmail},
		{ID: "hook", Type: ActionCallWebhook},
	}
	var order []string
	details, err := RunActionGraph(actions, func(a Action) (string, error) {
		order = append(order, a.ID)
		if a.ID == "insert" {
			return "insert:fail", errors.New("db down")
		}
		return a.ID + ":ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "insert" || order[1] != "hook" {
		t.Fatalf("order=%v", order)
	}
	if len(details) < 2 {
		t.Fatalf("details=%v", details)
	}
}

func TestRunActionGraphSuccessPath(t *testing.T) {
	actions := NormalizeActions([]Action{
		{Type: ActionDBInsert, Params: map[string]any{"database_id": "d", "table": "t"}},
		{Type: ActionSendEmail, Params: map[string]any{"template_key": "welcome"}},
	})
	var order []string
	_, err := RunActionGraph(actions, func(a Action) (string, error) {
		order = append(order, a.Type)
		return a.Type, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != ActionDBInsert || order[1] != ActionSendEmail {
		t.Fatalf("order=%v", order)
	}
}

func TestValidateCreateNormalizesWorkflow(t *testing.T) {
	spec, err := ValidateCreate(
		"app_user.created",
		json.RawMessage(`{"all":[]}`),
		json.RawMessage(`[
			{"type":"db_insert","params":{"database_id":"db1","table":"events"}},
			{"type":"send_email","params":{"template_key":"welcome"}}
		]`),
	)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Actions[0].OnSuccess != spec.Actions[1].ID {
		t.Fatalf("expected linear on_success, got %#v", spec.Actions)
	}
}
