package token

import (
	"testing"
)

// TestGetAADAccessToken_TenantIDFromEnv verifies we read AZURE_TENANT_ID from environment.
// Requires real Azure credentials so is skipped in short mode.
func TestGetAADAccessToken_TenantIDFromEnv(t *testing.T) {

	testTenantID := "test-tenant-123"
	t.Setenv("AZURE_TENANT_ID", testTenantID)

	ctx := t.Context()
	resp, _ := GetAADAccessToken(ctx)

	if resp.TenantID != testTenantID {
		t.Errorf("expected tenant ID %s from environment, got: %s", testTenantID, resp.TenantID)
	}
}
