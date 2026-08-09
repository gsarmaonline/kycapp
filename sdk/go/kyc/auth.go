// Package kyc is the transport layer for the KYC merchant Integration API.
//
// generated.go is produced from web/public/openapi-integration.yaml. Do not
// hand-edit it; run `make sdk` at the repo root instead.
//
// This package is deliberately thin. It exposes the generated client and types,
// and nothing else. The ergonomic facade (organisation bound at construction,
// cached entitlement checks) is a separate layer.
package kyc

import (
	"context"
	"net/http"
)

// WithAPIKey authenticates every request with an organisation API key (kyc_…).
//
//	client, err := kyc.NewClient("https://kyc.example.com", kyc.WithAPIKey(key))
func WithAPIKey(key string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+key)
		return nil
	})
}
