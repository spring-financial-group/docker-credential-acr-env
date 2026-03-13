package token

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type fakeTokenProvider struct {
	gotScopes []string
	err       error
}

func (f *fakeTokenProvider) GetToken(_ context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.gotScopes = opts.Scopes
	if f.err != nil {
		return azcore.AccessToken{}, f.err
	}
	return azcore.AccessToken{Token: "fake-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestGetAADAccessToken_TenantIDFromEnv(t *testing.T) {
	t.Setenv("AZURE_TENANT_ID", "test-tenant-123")

	resp, err := GetAADAccessToken(t.Context(), &fakeTokenProvider{}, "https://containerregistry.azure.net/.default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TenantID != "test-tenant-123" {
		t.Errorf("expected tenant ID test-tenant-123, got: %s", resp.TenantID)
	}

	if resp.AccessToken != "fake-token" {
		t.Errorf("expected fake-token, got: %s", resp.AccessToken)
	}
}

// TestGetAADAccessToken_RequestsScope pins that the caller-supplied scope is
// what gets requested — the ACR audience scope selection lives upstream and
// would be silently broken if this stopped passing it through.
func TestGetAADAccessToken_RequestsScope(t *testing.T) {
	tp := &fakeTokenProvider{}

	_, err := GetAADAccessToken(t.Context(), tp, "https://containerregistry.azure.net/.default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tp.gotScopes) != 1 || tp.gotScopes[0] != "https://containerregistry.azure.net/.default" {
		t.Errorf("requested scopes = %v, want exactly the caller's scope", tp.gotScopes)
	}
}

func TestGetAADAccessToken_WrapsProviderError(t *testing.T) {
	_, err := GetAADAccessToken(t.Context(), &fakeTokenProvider{err: errors.New("credential exploded")}, "any-scope")
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "failed to fetch AAD token") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "credential exploded") {
		t.Errorf("underlying cause missing from error: %v", err)
	}
}
