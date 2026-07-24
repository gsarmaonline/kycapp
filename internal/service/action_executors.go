package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/core/automations"
	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/core/resources"
	"github.com/gsarmaonline/kyc/internal/mailer"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

// actionExecutor runs a validated action with resolved subjects.
type actionExecutor func(
	ctx context.Context,
	s *Service,
	orgID string,
	params map[string]any,
	payload map[string]any,
	subjects map[string]map[string]any,
) (string, error)

func (s *Service) executeAction(
	ctx context.Context,
	orgID string,
	a automations.Action,
	payload map[string]any,
	subjects map[string]map[string]any,
) (string, error) {
	a = a.Normalize()
	if err := automations.ValidateAction(a); err != nil {
		return "", err
	}
	exec, ok := actionExecutors[a.Type]
	if !ok {
		return "", fmt.Errorf("unsupported action %q", a.Type)
	}
	return exec(ctx, s, orgID, a.Params, payload, subjects)
}

var actionExecutors = map[string]actionExecutor{
	automations.ActionSendEmail:    execSendEmail,
	automations.ActionCallWebhook:  execCallWebhook,
	automations.ActionDBInsert:     execDBInsert,
}

func execSendEmail(
	ctx context.Context,
	s *Service,
	orgID string,
	params map[string]any,
	_ map[string]any,
	subjects map[string]map[string]any,
) (string, error) {
	templateKey, err := automations.RequireStringParam(params, "template_key")
	if err != nil {
		return "", err
	}
	appUser, ok := subjects[resources.SubjectAppUser]
	if !ok || appUser == nil {
		return "", fmt.Errorf("send_email: requires an app_user subject from the trigger")
	}
	to := stringifyPayload(appUser["email"])
	if to == "" {
		return "", fmt.Errorf("send_email: app_user email is required")
	}
	if err := s.EnsureDefaultEmailTemplates(ctx, orgID); err != nil {
		return "", err
	}
	tmpl, err := s.db.Q().GetEmailTemplateByOrgKey(ctx, sqlc.GetEmailTemplateByOrgKeyParams{
		OrganisationID: orgID,
		Key:            templateKey,
	})
	if err != nil {
		return "", fmt.Errorf("email template %q not found", templateKey)
	}
	org, err := s.GetOrganisation(ctx, orgID)
	if err != nil {
		return "", err
	}
	vars := map[string]string{
		"display_name": stringifyPayload(appUser["display_name"]),
		"org_name":     org.Name,
		"email":        to,
	}
	subject := emailtemplates.Render(tmpl.Subject, vars)
	textBody := emailtemplates.Render(tmpl.BodyText, vars)
	htmlInner := emailtemplates.Render(tmpl.BodyHtml, vars)
	if strings.TrimSpace(htmlInner) == "" {
		htmlInner = "<p>" + html.EscapeString(textBody) + "</p>"
	}
	htmlBody := emailtemplates.Wrap(htmlInner, BrandingFromOrg(org))
	ref, err := s.mailer.Send(ctx, mailer.Message{
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
		Tags: map[string]string{
			"org_id":       orgID,
			"template_key": templateKey,
			"source":       "automation",
		},
	})
	if err != nil {
		return "", fmt.Errorf("send_email: %w", err)
	}
	slog.Info("automation send_email",
		"org_id", orgID,
		"template", templateKey,
		"provider", s.mailer.Name(),
		"provider_ref", ref,
		"to", to,
	)
	return "send_email:" + templateKey + " → " + to + " (" + s.mailer.Name() + ":" + ref + ")", nil
}

func execCallWebhook(
	ctx context.Context,
	s *Service,
	orgID string,
	params map[string]any,
	payload map[string]any,
	_ map[string]map[string]any,
) (string, error) {
	rawURL, err := automations.RequireStringParam(params, "url")
	if err != nil {
		return "", err
	}
	if err := assertPublicHTTPURL(rawURL); err != nil {
		return "", fmt.Errorf("call_webhook: %w", err)
	}
	secret := ""
	if v, ok := params["secret"]; ok && v != nil {
		secret = strings.TrimSpace(fmt.Sprint(v))
	}
	bodyObj := map[string]any{
		"organisation_id": orgID,
		"payload":         payload,
	}
	body, err := json.Marshal(bodyObj)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "kyc-automations/1")
	if secret != "" {
		req.Header.Set("X-KYC-Webhook-Secret", secret)
	}
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DialContext: restrictPublicDialContext,
			Proxy:       http.ProxyFromEnvironment,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call_webhook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("call_webhook: remote status %d", resp.StatusCode)
	}
	slog.Info("automation call_webhook", "org_id", orgID, "url", rawURL, "status", resp.StatusCode)
	return fmt.Sprintf("call_webhook:%s → %d", rawURL, resp.StatusCode), nil
}

func execDBInsert(
	ctx context.Context,
	s *Service,
	orgID string,
	params map[string]any,
	payload map[string]any,
	_ map[string]map[string]any,
) (string, error) {
	dbID, err := automations.RequireStringParam(params, "database_id")
	if err != nil {
		return "", err
	}
	tableRaw, err := automations.RequireStringParam(params, "table")
	if err != nil {
		return "", err
	}
	tableSQL, err := automations.ParseSQLIdentifier(tableRaw)
	if err != nil {
		return "", fmt.Errorf("db_insert: table: %w", err)
	}
	mapping, err := automations.ParseColumnMapping(params["mapping"])
	if err != nil {
		return "", fmt.Errorf("db_insert: %w", err)
	}
	row, err := s.organisationDatabaseRow(ctx, orgID, dbID)
	if err != nil {
		return "", fmt.Errorf("db_insert: %w", err)
	}
	conn, err := pgx.Connect(ctx, postgresDSN(row))
	if err != nil {
		return "", fmt.Errorf("db_insert: connect: %w", err)
	}
	defer conn.Close(ctx)

	var sql string
	var args []any
	if len(mapping) == 0 {
		trigger := stringifyPayload(payload["trigger"])
		if trigger == "" {
			// Worker payload may not include trigger; still dump payload.
			trigger = ""
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		sql = fmt.Sprintf(`INSERT INTO %s ("trigger", payload) VALUES ($1, $2::jsonb)`, tableSQL)
		args = []any{trigger, payloadJSON}
	} else {
		cols := make([]string, 0, len(mapping))
		for col := range mapping {
			cols = append(cols, col)
		}
		sort.Strings(cols)
		ph := make([]string, 0, len(cols))
		args = make([]any, 0, len(cols))
		for i, col := range cols {
			ph = append(ph, fmt.Sprintf("$%d", i+1))
			val, _ := lookupPayloadPath(payload, mapping[col])
			args = append(args, val)
			cols[i] = `"` + col + `"`
		}
		sql = fmt.Sprintf(
			`INSERT INTO %s (%s) VALUES (%s)`,
			tableSQL,
			strings.Join(cols, ", "),
			strings.Join(ph, ", "),
		)
	}
	tag, err := conn.Exec(ctx, sql, args...)
	if err != nil {
		return "", fmt.Errorf("db_insert: %w", err)
	}
	detail := fmt.Sprintf("db_insert:%s.%s rows=%d", row.Name, tableRaw, tag.RowsAffected())
	slog.Info("automation db_insert", "org_id", orgID, "database_id", dbID, "table", tableRaw, "rows", tag.RowsAffected())
	return detail, nil
}

func lookupPayloadPath(payload map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = payload
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func assertPublicHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("url is invalid")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return fmt.Errorf("url must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("url host is required")
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return fmt.Errorf("url host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("url host is not allowed")
		}
	}
	return nil
}

func restrictPublicDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	var last error
	d := net.Dialer{Timeout: 10 * time.Second}
	for _, ipa := range ips {
		if !isPublicIP(ipa.IP) {
			last = fmt.Errorf("resolved to non-public address")
			continue
		}
		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("no addresses")
	}
	return nil, last
}

func isPublicIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	// Cloud metadata
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return false
		}
	}
	return true
}

// resolveSubjects builds subject maps for action execution from the trigger payload
// and declared resource relations.
func (s *Service) resolveSubjects(ctx context.Context, orgID, trigger string, payload map[string]any) (map[string]map[string]any, error) {
	tr, err := resources.ParseTrigger(trigger)
	if err != nil {
		return nil, err
	}
	res, ok := resources.ByKey(tr.Resource)
	if !ok {
		return nil, fmt.Errorf("unknown resource %q", tr.Resource)
	}
	subjects := map[string]map[string]any{
		tr.Resource: payload,
	}
	for _, kind := range res.Provides {
		if _, exists := subjects[kind]; !exists {
			subjects[kind] = payload
		}
	}
	for _, rel := range res.Relations {
		if _, exists := subjects[rel.Subject]; exists {
			continue
		}
		id := stringifyPayload(payload[rel.Via])
		if id == "" {
			return nil, fmt.Errorf("cannot resolve subject %q: payload.%s is empty", rel.Subject, rel.Via)
		}
		resolved, err := s.loadSubject(ctx, orgID, rel.Subject, id)
		if err != nil {
			return nil, err
		}
		subjects[rel.Subject] = resolved
	}
	return subjects, nil
}

func (s *Service) loadSubject(ctx context.Context, orgID, kind, id string) (map[string]any, error) {
	switch kind {
	case resources.SubjectUser:
		u, err := s.GetUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("resolve user %q: %w", id, err)
		}
		return map[string]any{
			"id":     u.ID,
			"email":  u.Email,
			"name":   u.Name,
			"status": u.Status,
		}, nil
	case resources.SubjectAppUser:
		u, err := s.GetAppUser(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("resolve app_user %q: %w", id, err)
		}
		if u.OrganisationID != orgID {
			return nil, fmt.Errorf("app_user %q does not belong to organisation", id)
		}
		return AppUserEventPayload(u)
	default:
		return nil, fmt.Errorf("no resolver for subject %q", kind)
	}
}
