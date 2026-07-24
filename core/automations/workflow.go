package automations

import (
	"errors"
	"fmt"
	"strings"
)

const maxWorkflowSteps = 64

// ErrActionPaused means the workflow intentionally stopped to resume later
// (e.g. delay). It is not a failure — do not follow on_error.
var ErrActionPaused = errors.New("action paused")

// NormalizeActions assigns stable ids and chains on_success linearly when no
// explicit edges are present (legacy flat lists become a success path).
func NormalizeActions(actions []Action) []Action {
	if len(actions) == 0 {
		return actions
	}
	out := make([]Action, len(actions))
	for i, a := range actions {
		a = a.Normalize()
		if a.ID == "" {
			a.ID = fmt.Sprintf("a%d", i+1)
		}
		out[i] = a
	}
	// Dedupe ids if needed.
	seen := map[string]int{}
	for i := range out {
		id := out[i].ID
		if n, ok := seen[id]; ok {
			out[i].ID = fmt.Sprintf("%s_%d", id, n+1)
			seen[id] = n + 1
		} else {
			seen[id] = 1
		}
	}

	hasEdges := false
	for _, a := range out {
		if a.OnSuccess != "" || a.OnError != "" {
			hasEdges = true
			break
		}
	}
	if !hasEdges {
		for i := 0; i < len(out)-1; i++ {
			out[i].OnSuccess = out[i+1].ID
		}
	}
	return out
}

// ValidateActionGraph checks ids and edge targets after NormalizeActions.
func ValidateActionGraph(actions []Action) error {
	if len(actions) == 0 {
		return fmt.Errorf("at least one action is required")
	}
	byID := make(map[string]Action, len(actions))
	for i, a := range actions {
		id := strings.TrimSpace(a.ID)
		if id == "" {
			return fmt.Errorf("actions[%d].id is required", i)
		}
		if _, exists := byID[id]; exists {
			return fmt.Errorf("duplicate action id %q", id)
		}
		byID[id] = a
	}
	for i, a := range actions {
		if a.OnSuccess != "" {
			if _, ok := byID[a.OnSuccess]; !ok {
				return fmt.Errorf("actions[%d].on_success %q is unknown", i, a.OnSuccess)
			}
			if a.OnSuccess == a.ID {
				return fmt.Errorf("actions[%d].on_success cannot point to itself", i)
			}
		}
		if a.OnError != "" {
			if _, ok := byID[a.OnError]; !ok {
				return fmt.Errorf("actions[%d].on_error %q is unknown", i, a.OnError)
			}
			if a.OnError == a.ID {
				return fmt.Errorf("actions[%d].on_error cannot point to itself", i)
			}
		}
	}
	// Entry must reach at least itself; detect cycles on success path from entry.
	if err := detectCycle(actions[0].ID, byID, true); err != nil {
		return err
	}
	return nil
}

func detectCycle(entry string, byID map[string]Action, successOnly bool) error {
	seen := map[string]bool{}
	stack := map[string]bool{}
	var visit func(id string) error
	visit = func(id string) error {
		if stack[id] {
			return fmt.Errorf("action workflow cycle involving %q", id)
		}
		if seen[id] {
			return nil
		}
		a, ok := byID[id]
		if !ok {
			return nil
		}
		stack[id] = true
		seen[id] = true
		if a.OnSuccess != "" {
			if err := visit(a.OnSuccess); err != nil {
				return err
			}
		}
		if !successOnly && a.OnError != "" {
			if err := visit(a.OnError); err != nil {
				return err
			}
		}
		delete(stack, id)
		return nil
	}
	if err := visit(entry); err != nil {
		return err
	}
	// Also walk error branches from reachable success nodes.
	for id := range seen {
		a := byID[id]
		if a.OnError != "" {
			if err := visit(a.OnError); err != nil {
				return err
			}
		}
	}
	return nil
}

// WalkActions yields action ids in execution order for one run path.
// onResult(id) should execute the action and return error if it failed.
// Returns the list of executed step details via onResult's side effects;
// the walker returns the terminal error (if any) after optional on_error hop.
type ActionRunner func(a Action) (detail string, err error)

// RunActionGraph walks from the first action following on_success / on_error.
func RunActionGraph(actions []Action, run ActionRunner) (details []string, err error) {
	if len(actions) == 0 {
		return nil, fmt.Errorf("no actions")
	}
	return RunActionGraphFrom(actions, actions[0].ID, run)
}

// RunActionGraphFrom walks starting at startID (for resume after delay).
func RunActionGraphFrom(actions []Action, startID string, run ActionRunner) (details []string, err error) {
	if len(actions) == 0 {
		return nil, fmt.Errorf("no actions")
	}
	startID = strings.TrimSpace(startID)
	if startID == "" {
		startID = actions[0].ID
	}
	byID := make(map[string]Action, len(actions))
	for _, a := range actions {
		byID[a.ID] = a
	}
	if _, ok := byID[startID]; !ok {
		return nil, fmt.Errorf("unknown start action id %q", startID)
	}
	cur := startID
	visited := map[string]int{}
	for steps := 0; cur != "" && steps < maxWorkflowSteps; steps++ {
		if visited[cur] > 0 {
			return details, fmt.Errorf("action workflow cycle at %q", cur)
		}
		visited[cur]++
		a, ok := byID[cur]
		if !ok {
			return details, fmt.Errorf("unknown action id %q", cur)
		}
		detail, runErr := run(a)
		if detail != "" {
			details = append(details, detail)
		}
		if runErr != nil {
			if errors.Is(runErr, ErrActionPaused) {
				return details, ErrActionPaused
			}
			if a.OnError == "" {
				return details, runErr
			}
			details = append(details, fmt.Sprintf("%s:error→%s", a.ID, a.OnError))
			cur = a.OnError
			continue
		}
		cur = a.OnSuccess
	}
	if cur != "" {
		return details, fmt.Errorf("action workflow exceeded %d steps", maxWorkflowSteps)
	}
	return details, nil
}
