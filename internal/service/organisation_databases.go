package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5"
)

const (
	DatabaseStatusConnected    = "connected"
	DatabaseStatusUnreachable  = "unreachable"
	DatabaseStatusDisconnected = "disconnected"
)

// OrganisationDatabaseView is the API-safe view (password masked).
type OrganisationDatabaseView struct {
	ID             string
	OrganisationID string
	Name           string
	Driver         string
	Host           string
	Port           int32
	DatabaseName   string
	Username       string
	PasswordHint   string
	HasPassword    bool
	SSLMode        string
	Status         string
	LastCheckedAt  *time.Time
	LastError      string
}

type CreateOrganisationDatabaseInput struct {
	Name         string
	Host         string
	Port         int32
	DatabaseName string
	Username     string
	Password     string
	SSLMode      string
}

type UpdateOrganisationDatabaseInput struct {
	Name         *string
	Host         *string
	Port         *int32
	DatabaseName *string
	Username     *string
	Password     *string // empty/nil keeps existing
	SSLMode      *string
}

func organisationDatabaseView(row sqlc.OrganisationDatabase) OrganisationDatabaseView {
	var checked *time.Time
	if row.LastCheckedAt.Valid {
		t := row.LastCheckedAt.Time
		checked = &t
	}
	return OrganisationDatabaseView{
		ID:             row.ID,
		OrganisationID: row.OrganisationID,
		Name:           row.Name,
		Driver:         row.Driver,
		Host:           row.Host,
		Port:           row.Port,
		DatabaseName:   row.DatabaseName,
		Username:       row.Username,
		PasswordHint:   maskSecret(row.Password),
		HasPassword:    strings.TrimSpace(row.Password) != "",
		SSLMode:        row.SslMode,
		Status:         row.Status,
		LastCheckedAt:  checked,
		LastError:      row.LastError,
	}
}

func (s *Service) ListOrganisationDatabases(ctx context.Context, orgID string) ([]OrganisationDatabaseView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, err
	}
	rows, err := s.db.Q().ListOrganisationDatabases(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]OrganisationDatabaseView, 0, len(rows))
	for _, row := range rows {
		out = append(out, organisationDatabaseView(row))
	}
	return out, nil
}

func (s *Service) GetOrganisationDatabase(ctx context.Context, orgID, id string) (OrganisationDatabaseView, error) {
	row, err := s.db.Q().GetOrganisationDatabaseForOrg(ctx, sqlc.GetOrganisationDatabaseForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationDatabaseView{}, mapNotFound(err, "database not found")
	}
	return organisationDatabaseView(row), nil
}

func (s *Service) CreateOrganisationDatabase(ctx context.Context, orgID string, in CreateOrganisationDatabaseInput) (OrganisationDatabaseView, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return OrganisationDatabaseView{}, err
	}
	name := strings.TrimSpace(in.Name)
	host := strings.TrimSpace(in.Host)
	dbName := strings.TrimSpace(in.DatabaseName)
	user := strings.TrimSpace(in.Username)
	if name == "" {
		return OrganisationDatabaseView{}, apperr.Validation("name is required")
	}
	if host == "" {
		return OrganisationDatabaseView{}, apperr.Validation("host is required")
	}
	if dbName == "" {
		return OrganisationDatabaseView{}, apperr.Validation("database_name is required")
	}
	if user == "" {
		return OrganisationDatabaseView{}, apperr.Validation("username is required")
	}
	if strings.TrimSpace(in.Password) == "" {
		return OrganisationDatabaseView{}, apperr.Validation("password is required")
	}
	port := in.Port
	if port == 0 {
		port = 5432
	}
	if port < 1 || port > 65535 {
		return OrganisationDatabaseView{}, apperr.Validation("port is invalid")
	}
	ssl := strings.TrimSpace(in.SSLMode)
	if ssl == "" {
		ssl = "require"
	}
	row, err := s.db.Q().CreateOrganisationDatabase(ctx, sqlc.CreateOrganisationDatabaseParams{
		ID:             ids.New(),
		OrganisationID: orgID,
		Name:           name,
		Driver:         "postgres",
		Host:           host,
		Port:           port,
		DatabaseName:   dbName,
		Username:       user,
		Password:       in.Password,
		SslMode:        ssl,
		Status:         DatabaseStatusUnreachable,
		LastError:      "",
	})
	if err != nil {
		return OrganisationDatabaseView{}, err
	}
	return s.applyDatabaseCheck(ctx, row)
}

func (s *Service) UpdateOrganisationDatabase(ctx context.Context, orgID, id string, in UpdateOrganisationDatabaseInput) (OrganisationDatabaseView, error) {
	existing, err := s.db.Q().GetOrganisationDatabaseForOrg(ctx, sqlc.GetOrganisationDatabaseForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationDatabaseView{}, mapNotFound(err, "database not found")
	}
	name := existing.Name
	host := existing.Host
	port := existing.Port
	dbName := existing.DatabaseName
	user := existing.Username
	ssl := existing.SslMode
	password := "" // empty keeps existing in SQL
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if name == "" {
			return OrganisationDatabaseView{}, apperr.Validation("name is required")
		}
	}
	if in.Host != nil {
		host = strings.TrimSpace(*in.Host)
		if host == "" {
			return OrganisationDatabaseView{}, apperr.Validation("host is required")
		}
	}
	if in.Port != nil {
		port = *in.Port
		if port < 1 || port > 65535 {
			return OrganisationDatabaseView{}, apperr.Validation("port is invalid")
		}
	}
	if in.DatabaseName != nil {
		dbName = strings.TrimSpace(*in.DatabaseName)
		if dbName == "" {
			return OrganisationDatabaseView{}, apperr.Validation("database_name is required")
		}
	}
	if in.Username != nil {
		user = strings.TrimSpace(*in.Username)
		if user == "" {
			return OrganisationDatabaseView{}, apperr.Validation("username is required")
		}
	}
	if in.Password != nil {
		password = *in.Password
	}
	if in.SSLMode != nil {
		ssl = strings.TrimSpace(*in.SSLMode)
		if ssl == "" {
			ssl = "require"
		}
	}
	row, err := s.db.Q().UpdateOrganisationDatabase(ctx, sqlc.UpdateOrganisationDatabaseParams{
		ID:             id,
		OrganisationID: orgID,
		Name:           name,
		Host:           host,
		Port:           port,
		DatabaseName:   dbName,
		Username:       user,
		Password:       password,
		SslMode:        ssl,
	})
	if err != nil {
		return OrganisationDatabaseView{}, err
	}
	return s.applyDatabaseCheck(ctx, row)
}

// CheckOrganisationDatabase probes connectivity and updates status.
func (s *Service) CheckOrganisationDatabase(ctx context.Context, orgID, id string) (OrganisationDatabaseView, error) {
	row, err := s.db.Q().GetOrganisationDatabaseForOrg(ctx, sqlc.GetOrganisationDatabaseForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationDatabaseView{}, mapNotFound(err, "database not found")
	}
	return s.applyDatabaseCheck(ctx, row)
}

// SetOrganisationDatabaseDisconnected marks the connection disabled without probing.
func (s *Service) SetOrganisationDatabaseDisconnected(ctx context.Context, orgID, id string) (OrganisationDatabaseView, error) {
	_, err := s.db.Q().GetOrganisationDatabaseForOrg(ctx, sqlc.GetOrganisationDatabaseForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return OrganisationDatabaseView{}, mapNotFound(err, "database not found")
	}
	row, err := s.db.Q().UpdateOrganisationDatabaseCheck(ctx, sqlc.UpdateOrganisationDatabaseCheckParams{
		ID:             id,
		OrganisationID: orgID,
		Status:         DatabaseStatusDisconnected,
		LastError:      "manually disconnected",
	})
	if err != nil {
		return OrganisationDatabaseView{}, err
	}
	return organisationDatabaseView(row), nil
}

func (s *Service) applyDatabaseCheck(ctx context.Context, row sqlc.OrganisationDatabase) (OrganisationDatabaseView, error) {
	status := DatabaseStatusConnected
	lastErr := ""
	if err := probePostgres(ctx, row); err != nil {
		status = DatabaseStatusUnreachable
		lastErr = truncateErr(err.Error(), 500)
	}
	updated, err := s.db.Q().UpdateOrganisationDatabaseCheck(ctx, sqlc.UpdateOrganisationDatabaseCheckParams{
		ID:             row.ID,
		OrganisationID: row.OrganisationID,
		Status:         status,
		LastError:      lastErr,
	})
	if err != nil {
		return OrganisationDatabaseView{}, err
	}
	return organisationDatabaseView(updated), nil
}

func (s *Service) DeleteOrganisationDatabase(ctx context.Context, orgID, id string) error {
	if _, err := s.GetOrganisationDatabase(ctx, orgID, id); err != nil {
		return err
	}
	return s.db.Q().DeleteOrganisationDatabase(ctx, sqlc.DeleteOrganisationDatabaseParams{
		ID: id, OrganisationID: orgID,
	})
}

func (s *Service) organisationDatabaseRow(ctx context.Context, orgID, id string) (sqlc.OrganisationDatabase, error) {
	row, err := s.db.Q().GetOrganisationDatabaseForOrg(ctx, sqlc.GetOrganisationDatabaseForOrgParams{
		ID: id, OrganisationID: orgID,
	})
	if err != nil {
		return sqlc.OrganisationDatabase{}, mapNotFound(err, "database not found")
	}
	if row.Status != DatabaseStatusConnected {
		return sqlc.OrganisationDatabase{}, apperr.Validation("database is not connected (status=" + row.Status + ")")
	}
	if strings.TrimSpace(row.Password) == "" {
		return sqlc.OrganisationDatabase{}, apperr.Validation("database password is not set")
	}
	return row, nil
}

func probePostgres(ctx context.Context, row sqlc.OrganisationDatabase) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, postgresDSN(row))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	var one int
	if err := conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		return err
	}
	return nil
}

func truncateErr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func postgresDSN(row sqlc.OrganisationDatabase) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(row.Username, row.Password),
		Host:   net.JoinHostPort(row.Host, fmt.Sprintf("%d", row.Port)),
		Path:   "/" + row.DatabaseName,
	}
	q := url.Values{}
	ssl := strings.TrimSpace(row.SslMode)
	if ssl == "" {
		ssl = "require"
	}
	q.Set("sslmode", ssl)
	q.Set("connect_timeout", "8")
	u.RawQuery = q.Encode()
	return u.String()
}
