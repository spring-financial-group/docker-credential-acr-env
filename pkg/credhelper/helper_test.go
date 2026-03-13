/*
Copyright © 2022 Chris Mellard chris.mellard@icloud.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package credhelper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/spring-financial-group/docker-credential-acr-env/pkg/token"
)

// TestIsACRRegistry tests our URL validation logic - the core of what this helper does
func TestIsACRRegistry(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		// Valid ACR registries
		{"myregistry.azurecr.io", true},
		{"myregistry.azurecr.cn", true},
		{"myregistry.azurecr.us", true},
		{"mcr.microsoft.com", true},

		// Hostnames are case-insensitive
		{"MYREG.AZURECR.IO", true},
		{"MCR.Microsoft.COM", true},

		// Suffix confusion: the ACR suffix must terminate the hostname
		{"myregistry.azurecr.io.evil.com", false},
		{"evil.azurecr.cn.attacker.net", false},
		{"azurecr.io.attacker.com", false},
		{"mcr.microsoft.com.evil.com", false},

		// Not ACR
		{"myregistry.azurecr.de", false}, // Azure Germany is retired; no cloud.Configuration exists
		{"myregistry.azurecr.me", false},
		{"docker.io", false},
		{"gcr.io", false},
		{"localhost:5000", false},
		{"notacr.xcr.example", false},
		{"127.0.0.1:12345", false},
		{"localhost:12345", false},
		{"notaurl-)(*$@)(*@)(*", false}, // exercises the url.Parse error branch
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isACRRegistry(tt.url)
			if got != tt.want {
				t.Errorf("isACRRegistry(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// fakeTokenProvider returns a fixed token, or a fixed error, and captures the
// context and scopes it was called with.
type fakeTokenProvider struct {
	token    string
	err      error
	gotCtx   context.Context
	gotScope []string
}

func (f *fakeTokenProvider) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	f.gotCtx = ctx
	f.gotScope = opts.Scopes
	if f.err != nil {
		return azcore.AccessToken{}, f.err
	}
	return azcore.AccessToken{Token: f.token, ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// reroutingTransport rewrites every request onto the test server.
type reroutingTransport struct {
	target *url.URL
}

func (rt reroutingTransport) Do(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// newFakeRegistry starts a fake ACR exchange endpoint and returns exchange
// client options routed to it.
func newFakeRegistry(t *testing.T, handler http.HandlerFunc) *azcontainerregistry.AuthenticationClientOptions {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	return &azcontainerregistry.AuthenticationClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: reroutingTransport{target: target},
			Retry:     policy.RetryOptions{MaxRetries: 0},
		},
	}
}

func newTestHelper(tp token.TokenProvider, exchangeOpts *azcontainerregistry.AuthenticationClientOptions) ACRCredHelper {
	return ACRCredHelper{
		tokenProvider:   tp,
		cloud:           cloud.AzurePublic,
		exchangeOptions: exchangeOpts,
	}
}

func TestGetHappyPath(t *testing.T) {
	exchangeOpts := newFakeRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("failed to parse request body: %v", err)
		}
		if got := form.Get("access_token"); got != "fake-aad-token" {
			t.Errorf("exchange access_token = %q, want the AAD token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{"refresh_token":"fake-refresh-token"}`); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	tp := &fakeTokenProvider{token: "fake-aad-token"}
	h := newTestHelper(tp, exchangeOpts)

	user, secret, err := h.Get("myregistry.azurecr.io")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != tokenUsername {
		t.Errorf("username = %q, want %q", user, tokenUsername)
	}
	if secret != "fake-refresh-token" {
		t.Errorf("secret = %q, want the exchange refresh token", secret)
	}

	// The whole operation shares one deadline owned by Get.
	if _, ok := tp.gotCtx.Deadline(); !ok {
		t.Error("expected the context passed to the token provider to carry a deadline")
	}
	// The requested scope is derived from the cloud's ACR audience.
	if len(tp.gotScope) != 1 || !strings.Contains(tp.gotScope[0], "containerregistry.azure.net") {
		t.Errorf("requested scopes = %v, want the ACR audience scope", tp.gotScope)
	}
}

func TestGetRejectsNonACR(t *testing.T) {
	h := newTestHelper(&fakeTokenProvider{token: "unused"}, nil)

	_, _, err := h.Get("docker.io")
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "does not refer to Azure Container Registry") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetWrapsTokenAcquisitionFailure(t *testing.T) {
	h := newTestHelper(&fakeTokenProvider{err: errors.New("aad exploded")}, nil)

	_, _, err := h.Get("myregistry.azurecr.io")
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "failed to acquire AAD token") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "aad exploded") {
		t.Errorf("underlying cause missing from error: %v", err)
	}
}

func TestGetWrapsExchangeFailure(t *testing.T) {
	exchangeOpts := newFakeRegistry(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	h := newTestHelper(&fakeTokenProvider{token: "fake-aad-token"}, exchangeOpts)

	_, _, err := h.Get("myregistry.azurecr.io")
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "failed to acquire refresh token") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestNewACRCredentialsHelperRejectsUnknownCloud pins that a bad
// AZURE_ENVIRONMENT fails loudly at construction instead of silently
// authenticating against the public cloud.
func TestNewACRCredentialsHelperRejectsUnknownCloud(t *testing.T) {
	t.Setenv("AZURE_ENVIRONMENT", "not-a-real-cloud")

	_, err := NewACRCredentialsHelper()
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "not-a-real-cloud") {
		t.Errorf("error should name the offending value: %v", err)
	}
}

// TestUnimplementedOperations pins the docker credential helper contract for
// the operations this helper deliberately doesn't support.
func TestUnimplementedOperations(t *testing.T) {
	h := ACRCredHelper{}

	if err := h.Add(nil); err == nil {
		t.Error("Add: expected unimplemented error, got nil")
	}
	if err := h.Delete("myregistry.azurecr.io"); err == nil {
		t.Error("Delete: expected unimplemented error, got nil")
	}
	if _, err := h.List(); err == nil {
		t.Error("List: expected unimplemented error, got nil")
	}
}
