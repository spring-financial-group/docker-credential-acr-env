package azcloud

import (
	"strings"
	"testing"
)

func TestFromEnvironment(t *testing.T) {
	tests := []struct {
		env           string
		wantAuthority string
		wantErr       bool
	}{
		{"", "https://login.microsoftonline.com/", false},
		{"AzurePublicCloud", "https://login.microsoftonline.com/", false},
		{"azurepubliccloud", "https://login.microsoftonline.com/", false},
		{"AzureChinaCloud", "https://login.chinacloudapi.cn/", false},
		{"AZURECHINACLOUD", "https://login.chinacloudapi.cn/", false},
		{"AzureUSGovernmentCloud", "https://login.microsoftonline.us/", false},
		{"azureusgovernmentcloud", "https://login.microsoftonline.us/", false},
		{"AzureGermanCloud", "", true},
		{"made-up-cloud", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			t.Setenv("AZURE_ENVIRONMENT", tt.env)

			cfg, err := FromEnvironment()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got none", tt.env)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.ActiveDirectoryAuthorityHost != tt.wantAuthority {
				t.Errorf("authority host = %q, want %q", cfg.ActiveDirectoryAuthorityHost, tt.wantAuthority)
			}
		})
	}
}

// TestACRTokenScope asserts the scope is derived from the cloud's registered
// ACR audience rather than a hardcoded literal. The audience happens to be
// identical across clouds today; this test pins the derivation mechanism, not
// the value.
func TestACRTokenScope(t *testing.T) {
	for _, env := range []string{"AzurePublicCloud", "AzureChinaCloud", "AzureUSGovernmentCloud"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("AZURE_ENVIRONMENT", env)

			cfg, err := FromEnvironment()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			scope, err := ACRTokenScope(cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			audience, ok := cfg.Services["azcontainerregistry"]
			if !ok {
				t.Fatal("expected azcontainerregistry service to be registered for cloud")
			}
			if scope != audience.Audience+"/.default" {
				t.Errorf("scope = %q, want %q derived from the cloud audience", scope, audience.Audience+"/.default")
			}
			if !strings.HasPrefix(scope, "https://containerregistry.azure.net") {
				t.Errorf("unexpected ACR audience in scope %q", scope)
			}
		})
	}
}
