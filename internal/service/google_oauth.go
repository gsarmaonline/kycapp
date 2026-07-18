package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gsarmaonline/kyc/internal/apperr"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleOAuth exchanges authorization codes and fetches the user profile.
type GoogleOAuth struct {
	cfg *oauth2.Config
}

func NewGoogleOAuth(clientID, clientSecret, redirectURL string) *GoogleOAuth {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &GoogleOAuth{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (g *GoogleOAuth) Enabled() bool {
	return g != nil && g.cfg != nil
}

func (g *GoogleOAuth) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.SetAuthURLParam("prompt", "select_account"))
}

func (g *GoogleOAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	tok, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return nil, apperr.Unauthorized("failed to exchange google auth code")
	}
	return tok, nil
}

func (g *GoogleOAuth) FetchIdentity(ctx context.Context, tok *oauth2.Token) (GoogleIdentity, error) {
	client := g.cfg.Client(ctx, tok)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return GoogleIdentity{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return GoogleIdentity{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return GoogleIdentity{}, apperr.Unauthorized(fmt.Sprintf("google userinfo failed: %s", strings.TrimSpace(string(body))))
	}
	var raw struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return GoogleIdentity{}, err
	}
	verified := false
	switch v := raw.EmailVerified.(type) {
	case bool:
		verified = v
	case string:
		verified = v == "true"
	}
	return GoogleIdentity{
		Sub:           raw.Sub,
		Email:         raw.Email,
		EmailVerified: verified,
		Name:          raw.Name,
		Picture:       strings.TrimSpace(raw.Picture),
	}, nil
}
