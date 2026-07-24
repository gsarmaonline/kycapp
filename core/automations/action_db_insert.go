package automations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func init() {
	RegisterActionHandler(dbInsertHandler{})
}

const ActionDBInsert = "db_insert"

var identRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type dbInsertHandler struct{}

func (dbInsertHandler) Type() string { return ActionDBInsert }

func (dbInsertHandler) Info() ActionInfo {
	return ActionInfo{
		Type: ActionDBInsert,
		Label: "Insert into database",
		Description: "Write the event into an org database connection. " +
			"Without mapping: INSERT (trigger, payload). With mapping: map columns to payload paths.",
		Params: []ActionParam{
			{Key: "database_id", Label: "Database", Required: true},
			{Key: "table", Label: "Table", Required: true},
			{Key: "mode", Label: "Mode (event|columns)", Required: false},
			{Key: "mapping", Label: "Column mapping", Required: false},
		},
		Requires: nil,
	}
}

func (dbInsertHandler) Requires() []string { return nil }

func (dbInsertHandler) Validate(params map[string]any) error {
	if _, err := RequireStringParam(params, "database_id"); err != nil {
		return err
	}
	table, err := RequireStringParam(params, "table")
	if err != nil {
		return err
	}
	if _, err := ParseSQLIdentifier(table); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	mode := ""
	if v, ok := params["mode"]; ok && v != nil {
		mode = strings.TrimSpace(fmt.Sprint(v))
	}
	mapping, err := ParseColumnMapping(params["mapping"])
	if err != nil {
		return err
	}
	if mode == "columns" && len(mapping) == 0 {
		return fmt.Errorf("mapping is required when mode is columns")
	}
	if mode != "" && mode != "event" && mode != "columns" {
		return fmt.Errorf("mode must be event or columns")
	}
	return nil
}

// ParseSQLIdentifier validates schema.table or table (unquoted identifiers only).
func ParseSQLIdentifier(raw string) (quoted string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("is required")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("must be table or schema.table")
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if !identRE.MatchString(p) {
			return "", fmt.Errorf("%q is not a valid SQL identifier", p)
		}
		out = append(out, `"`+p+`"`)
	}
	return strings.Join(out, "."), nil
}

// ParseColumnMapping accepts nil/empty, a JSON object, or a JSON string object.
// Values are field paths (e.g. "app_user.email", "app_user.country").
func ParseColumnMapping(raw any) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	switch t := raw.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil, nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return nil, fmt.Errorf("mapping must be a JSON object")
		}
		return mappingFromAny(m)
	case map[string]any:
		return mappingFromAny(t)
	case map[string]string:
		out := make(map[string]string, len(t))
		for k, v := range t {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k == "" || v == "" {
				continue
			}
			if !identRE.MatchString(k) {
				return nil, fmt.Errorf("mapping column %q is not a valid identifier", k)
			}
			out[k] = v
		}
		return out, nil
	default:
		return nil, fmt.Errorf("mapping must be a JSON object")
	}
}

func mappingFromAny(m map[string]any) (map[string]string, error) {
	out := make(map[string]string, len(m))
	for k, v := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if !identRE.MatchString(k) {
			return nil, fmt.Errorf("mapping column %q is not a valid identifier", k)
		}
		path := strings.TrimSpace(fmt.Sprint(v))
		if path == "" {
			return nil, fmt.Errorf("mapping column %q needs a payload path", k)
		}
		out[k] = path
	}
	return out, nil
}
