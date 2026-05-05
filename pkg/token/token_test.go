package token

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type fakeTokenProvider struct{}

func (f *fakeTokenProvider) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

func TestGetAADAccessToken_TenantIDFromEnv(t *testing.T) {
	t.Setenv("AZURE_TENANT_ID", "test-tenant-123")

	resp, err := GetAADAccessToken(t.Context(), &fakeTokenProvider{})
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
