package mailer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendSend(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	t.Cleanup(srv.Close)

	m, err := NewResend("re_test", "KYC <mail@example.com>")
	if err != nil {
		t.Fatal(err)
	}
	m.apiURL = srv.URL
	m.client = srv.Client()

	ref, err := m.Send(context.Background(), Message{
		To:      []string{"user@example.com"},
		Subject: "Hello",
		HTML:    "<p>Hi</p>",
		Text:    "Hi",
		Tags:    map[string]string{"org_id": "org1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ref != "email_123" {
		t.Fatalf("ref=%q", ref)
	}
	if gotAuth != "Bearer re_test" {
		t.Fatalf("auth=%q", gotAuth)
	}
	if gotBody["from"] != "KYC <mail@example.com>" {
		t.Fatalf("from=%v", gotBody["from"])
	}
}

func TestNewResendRequiresConfig(t *testing.T) {
	if _, err := NewResend("", "a@b.com"); err == nil {
		t.Fatal("expected error for empty api key")
	}
	if _, err := NewResend("re_x", ""); err == nil {
		t.Fatal("expected error for empty from")
	}
}
