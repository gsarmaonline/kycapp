package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gsarmaonline/kyc/core/emailtemplates"
	"github.com/gsarmaonline/kyc/internal/apperr"
	"github.com/gsarmaonline/kyc/internal/store/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// MaxLogoBytes is the upload size cap for organisation logos.
const MaxLogoBytes = 1 << 20 // 1 MiB

func (s *Service) publicLogoURL(orgID string) string {
	return s.publicBaseURL + "/v1/public/organisations/" + orgID + "/branding/logo"
}

func (s *Service) logoPath(orgID string) string {
	return filepath.Join(s.uploadDir, orgID, "logo")
}

func validateOptionalColor(ptr *string) (pgtype.Text, error) {
	if ptr == nil {
		return pgtype.Text{}, nil
	}
	c, err := emailtemplates.NormalizeColor(*ptr)
	if err != nil {
		return pgtype.Text{}, apperr.Validation(err.Error())
	}
	return pgtype.Text{String: c, Valid: true}, nil
}

func (s *Service) UpdateOrganisationBranding(ctx context.Context, id string, in UpdateOrganisationInput) (sqlc.Organisation, error) {
	params := sqlc.UpdateOrganisationParams{ID: id}
	if in.Name != nil {
		params.Name = pgtype.Text{String: strings.TrimSpace(*in.Name), Valid: true}
	}
	if in.Status != nil {
		st := strings.TrimSpace(*in.Status)
		switch st {
		case "active", "suspended", "archived":
			params.Status = pgtype.Text{String: st, Valid: true}
		default:
			return sqlc.Organisation{}, apperr.Validation("invalid status")
		}
	}
	pc, err := validateOptionalColor(in.PrimaryColor)
	if err != nil {
		return sqlc.Organisation{}, err
	}
	params.PrimaryColor = pc
	ac, err := validateOptionalColor(in.AccentColor)
	if err != nil {
		return sqlc.Organisation{}, err
	}
	params.AccentColor = ac
	if in.EmailFooter != nil {
		params.EmailFooter = pgtype.Text{String: *in.EmailFooter, Valid: true}
	}
	if in.EmailFont != nil {
		font, err := emailtemplates.NormalizeFont(*in.EmailFont)
		if err != nil {
			return sqlc.Organisation{}, apperr.Validation(err.Error())
		}
		params.EmailFont = pgtype.Text{String: font, Valid: true}
	}
	org, err := s.db.Q().UpdateOrganisation(ctx, params)
	return org, mapNotFound(err, "organisation not found")
}

func (s *Service) SetOrganisationLogo(ctx context.Context, orgID string, r io.Reader, contentType string) (sqlc.Organisation, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return sqlc.Organisation{}, err
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(ct, "image/png"),
		strings.HasPrefix(ct, "image/jpeg"),
		strings.HasPrefix(ct, "image/jpg"),
		strings.HasPrefix(ct, "image/webp"):
	default:
		return sqlc.Organisation{}, apperr.Validation("logo must be png, jpeg, or webp")
	}

	dir := filepath.Join(s.uploadDir, orgID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		if os.IsPermission(err) {
			return sqlc.Organisation{}, apperr.Validation(
				"upload directory is not writable; check UPLOAD_DIR volume permissions",
			)
		}
		return sqlc.Organisation{}, fmt.Errorf("create upload dir: %w", err)
	}
	path := s.logoPath(orgID)
	f, err := os.Create(path)
	if err != nil {
		if os.IsPermission(err) {
			return sqlc.Organisation{}, apperr.Validation(
				"upload directory is not writable; check UPLOAD_DIR volume permissions",
			)
		}
		return sqlc.Organisation{}, fmt.Errorf("create logo file: %w", err)
	}
	defer f.Close()

	limited := io.LimitReader(r, MaxLogoBytes+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		_ = os.Remove(path)
		return sqlc.Organisation{}, fmt.Errorf("write logo: %w", err)
	}
	if n > MaxLogoBytes {
		_ = os.Remove(path)
		return sqlc.Organisation{}, apperr.Validation("logo must be at most 1MB")
	}

	// Persist detected type beside the file for public serve.
	_ = os.WriteFile(path+".ct", []byte(strings.Split(ct, ";")[0]), 0o644)

	org, err := s.db.Q().SetOrganisationLogoURL(ctx, sqlc.SetOrganisationLogoURLParams{
		ID:      orgID,
		LogoUrl: s.publicLogoURL(orgID),
	})
	return org, mapNotFound(err, "organisation not found")
}

func (s *Service) ClearOrganisationLogo(ctx context.Context, orgID string) (sqlc.Organisation, error) {
	path := s.logoPath(orgID)
	_ = os.Remove(path)
	_ = os.Remove(path + ".ct")
	org, err := s.db.Q().SetOrganisationLogoURL(ctx, sqlc.SetOrganisationLogoURLParams{
		ID:      orgID,
		LogoUrl: "",
	})
	return org, mapNotFound(err, "organisation not found")
}

func (s *Service) OpenOrganisationLogo(ctx context.Context, orgID string) (io.ReadCloser, string, error) {
	if _, err := s.GetOrganisation(ctx, orgID); err != nil {
		return nil, "", err
	}
	path := s.logoPath(orgID)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", apperr.NotFound("logo not found")
		}
		return nil, "", err
	}
	ct := "application/octet-stream"
	if b, err := os.ReadFile(path + ".ct"); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			ct = s
		}
	} else {
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		ct = http.DetectContentType(buf[:n])
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			_ = f.Close()
			return nil, "", err
		}
	}
	return f, ct, nil
}

func BrandingFromOrg(o sqlc.Organisation) emailtemplates.Branding {
	return emailtemplates.Branding{
		OrgName:      o.Name,
		LogoURL:      o.LogoUrl,
		PrimaryColor: o.PrimaryColor,
		AccentColor:  o.AccentColor,
		Footer:       o.EmailFooter,
		Font:         o.EmailFont,
	}
}
