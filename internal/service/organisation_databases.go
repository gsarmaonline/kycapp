package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/ids"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
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
	Status       *string
}

func organisationDatabaseView(row sqlc.OrganisationDatabase) OrganisationDatabaseView {
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
		Status:         "connected",
	})
	if err != nil {
		return OrganisationDatabaseView{}, err
	}
	return organisationDatabaseView(row), nil
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
	status := existing.Status
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
	if in.Status != nil {
		status = strings.TrimSpace(*in.Status)
		if status != "connected" && status != "disconnected" {
			return OrganisationDatabaseView{}, apperr.Validation("status must be connected or disconnected")
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
		Status:         status,
	})
	if err != nil {
		return OrganisationDatabaseView{}, err
	}
	return organisationDatabaseView(row), nil
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
	if row.Status != "connected" {
		return sqlc.OrganisationDatabase{}, apperr.Validation("database is disconnected")
	}
	if strings.TrimSpace(row.Password) == "" {
		return sqlc.OrganisationDatabase{}, apperr.Validation("database password is not set")
	}
	return row, nil
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
	q.Set("connect_timeout", "10")
	u.RawQuery = q.Encode()
	return u.String()
}
