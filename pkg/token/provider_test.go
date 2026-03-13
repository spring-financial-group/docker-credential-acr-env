package token

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func TestNewDefaultTokenProvider_SelectsInlineAssertion(t *testing.T) {
	t.Setenv(envFederatedToken, "fake-jwt")
	t.Setenv(envClientID, "fake-client")
	t.Setenv(envTenantID, "fake-tenant")

	tp, err := NewDefaultTokenProvider(cloud.AzurePublic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tp.(*azidentity.ClientAssertionCredential); !ok {
		t.Errorf("expected *azidentity.ClientAssertionCredential, got %T", tp)
	}
}

func TestNewDefaultTokenProvider_TokenFileTakesPrecedence(t *testing.T) {
	t.Setenv(envFederatedToken, "fake-jwt")
	t.Setenv(envFederatedTokenFile, "/nonexistent/token/file")
	t.Setenv(envClientID, "fake-client")
	t.Setenv(envTenantID, "fake-tenant")

	tp, err := NewDefaultTokenProvider(cloud.AzurePublic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tp.(*azidentity.DefaultAzureCredential); !ok {
		t.Errorf("expected *azidentity.DefaultAzureCredential when %s is set, got %T", envFederatedTokenFile, tp)
	}
}

func TestNewDefaultTokenProvider_InlineRequiresClientAndTenant(t *testing.T) {
	tests := []struct {
		name     string
		clientID string
		tenantID string
		wantErr  string
	}{
		{"missing client id", "", "fake-tenant", envClientID},
		{"missing tenant id", "fake-client", "", envTenantID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envFederatedToken, "fake-jwt")
			t.Setenv(envClientID, tt.clientID)
			t.Setenv(envTenantID, tt.tenantID)

			_, err := NewDefaultTokenProvider(cloud.AzurePublic)
			if err == nil {
				t.Fatal("expected error, got none")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not mention %s", err, tt.wantErr)
			}
		})
	}
}

// reroutingTransport rewrites every request onto the test server, so the
// credential's discovery and token calls never leave the test process.
type reroutingTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt reroutingTransport) Do(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return rt.base.RoundTrip(req)
}

// TestNewDefaultTokenProvider_InlineTokenFlow drives a full token acquisition
// against a fake AAD: it proves the inline AZURE_FEDERATED_TOKEN reaches the
// token endpoint as the client assertion, without ever being written to disk.
func TestNewDefaultTokenProvider_InlineTokenFlow(t *testing.T) {
	const (
		fakeJWT     = "fake-federated-jwt"
		fakeTenant  = "fake-tenant"
		fakeClient  = "fake-client"
		fakeScope   = "https://containerregistry.azure.net/.default"
		fakeAADTok  = "fake-aad-access-token"
		contentType = "application/json"
	)

	var sawAssertion string
	var sawScope string

	mux := http.NewServeMux()
	mux.HandleFunc("/common/discovery/instance", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"tenant_discovery_endpoint": "https://" + r.Host + "/" + fakeTenant + "/v2.0/.well-known/openid-configuration",
			"metadata":                  []any{},
		})
	})
	mux.HandleFunc("/"+fakeTenant+"/v2.0/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"authorization_endpoint": "https://" + r.Host + "/" + fakeTenant + "/oauth2/v2.0/authorize",
			"token_endpoint":         "https://" + r.Host + "/" + fakeTenant + "/oauth2/v2.0/token",
			"issuer":                 "https://" + r.Host + "/" + fakeTenant,
		})
	})
	mux.HandleFunc("/"+fakeTenant+"/oauth2/v2.0/token", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read token request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("failed to parse token request body: %v", err)
		}
		sawAssertion = form.Get("client_assertion")
		sawScope = form.Get("scope")
		if got := form.Get("grant_type"); got != "client_credentials" {
			t.Errorf("grant_type = %q, want client_credentials", got)
		}
		if got := form.Get("client_id"); got != fakeClient {
			t.Errorf("client_id = %q, want %q", got, fakeClient)
		}
		writeJSON(t, w, map[string]any{
			"token_type":   "Bearer",
			"expires_in":   3600,
			"access_token": fakeAADTok,
		})
	})
	// TLS because azidentity rejects non-https authority hosts.
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}

	t.Setenv(envFederatedToken, fakeJWT)
	t.Setenv(envClientID, fakeClient)
	t.Setenv(envTenantID, fakeTenant)

	// The cloud's authority host is pointed at the fake AAD; the transport
	// reroutes discovery traffic there too.
	cfg := cloud.Configuration{
		ActiveDirectoryAuthorityHost: srv.URL + "/",
		Services:                     map[cloud.ServiceName]cloud.ServiceConfiguration{},
	}
	tp, err := newInlineAssertionProvider(cfg, reroutingTransport{target: target, base: srv.Client().Transport})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := GetAADAccessToken(t.Context(), tp, fakeScope)
	if err != nil {
		t.Fatalf("token acquisition failed: %v", err)
	}
	if resp.AccessToken != fakeAADTok {
		t.Errorf("access token = %q, want %q", resp.AccessToken, fakeAADTok)
	}
	if sawAssertion != fakeJWT {
		t.Errorf("client_assertion = %q, want the inline %s value %q", sawAssertion, envFederatedToken, fakeJWT)
	}
	if !strings.Contains(sawScope, "containerregistry.azure.net") {
		t.Errorf("scope = %q, want the ACR scope", sawScope)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}
