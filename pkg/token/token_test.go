package token

import (
	"context"
	"os"
	"testing"
)

// TestGetAADAccessToken_TenantIDFromEnv verifies we read AZURE_TENANT_ID from environment
func TestGetAADAccessToken_TenantIDFromEnv(t *testing.T) {
	orig := os.Getenv("AZURE_TENANT_ID")
	defer func() {
		if orig != "" {
			os.Setenv("AZURE_TENANT_ID", orig)
		} else {
			os.Unsetenv("AZURE_TENANT_ID")
		}
	}()

	testTenantID := "test-tenant-123"
	os.Setenv("AZURE_TENANT_ID", testTenantID)

	ctx := context.Background()
	resp, _ := GetAADAccessToken(ctx)

	if resp.TenantID != testTenantID {
		t.Errorf("expected tenant ID %s from environment, got: %s", testTenantID, resp.TenantID)
	}
}
