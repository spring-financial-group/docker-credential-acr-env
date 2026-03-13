package registry

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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
)

// TestToEndpoint tests our URL normalization logic
func TestToEndpoint(t *testing.T) {
	tests := []struct {
		serverURL string
		want      string
	}{
		{"myregistry.azurecr.io", "https://myregistry.azurecr.io"},
		{"https://myregistry.azurecr.io", "https://myregistry.azurecr.io"},
		{"myregistry.azurecr.io:443", "https://myregistry.azurecr.io:443"},
		{"myregistry.azurecr.io/v2/", "https://myregistry.azurecr.io"},
	}

	for _, tt := range tests {
		t.Run(tt.serverURL, func(t *testing.T) {
			got, err := toEndpoint(tt.serverURL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// reroutingTransport rewrites every request onto the test server, so the
// exchange client can be exercised without a real registry.
type reroutingTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt reroutingTransport) Do(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = rt.target.Scheme
	req.URL.Host = rt.target.Host
	return rt.base.RoundTrip(req)
}

// newFakeExchange starts a fake ACR exchange endpoint and returns the client
// options that route to it.
func newFakeExchange(t *testing.T, handler func(form url.Values, w http.ResponseWriter, r *http.Request)) *azcontainerregistry.AuthenticationClientOptions {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("failed to parse request body %q: %v", body, err)
		}
		handler(form, w, r)
	}))
	t.Cleanup(srv.Close)

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse test server URL: %v", err)
	}
	return &azcontainerregistry.AuthenticationClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: reroutingTransport{target: target, base: http.DefaultTransport},
		},
	}
}

// fakeRefreshToken is the token every successful fake exchange returns.
const fakeRefreshToken = "fake-refresh-token"

func writeRefreshToken(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.WriteString(w, `{"refresh_token":"`+fakeRefreshToken+`"}`); err != nil {
		t.Errorf("failed to write response: %v", err)
	}
}

// TestExchangeTenantHandling pins regression: an empty tenant must be omitted
// from the form entirely, a set tenant transmitted as-is.
func TestExchangeTenantHandling(t *testing.T) {
	var lastForm url.Values
	opts := newFakeExchange(t, func(form url.Values, w http.ResponseWriter, _ *http.Request) {
		lastForm = form
		writeRefreshToken(t, w)
	})

	if _, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io", "fake-access", "", opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := lastForm["tenant"]; ok {
		t.Errorf("tenant should be omitted when empty, got form %v", lastForm)
	}

	if _, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io", "fake-access", "my-tenant", opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := lastForm.Get("tenant"); got != "my-tenant" {
		t.Errorf("tenant = %q, want %q", got, "my-tenant")
	}
}

// TestExchangeRequestShape pins the wire contract of the AAD→ACR exchange.
func TestExchangeRequestShape(t *testing.T) {
	var gotMethod, gotPath, gotAPIVersion string
	var gotForm url.Values
	opts := newFakeExchange(t, func(form url.Values, w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAPIVersion = r.URL.Query().Get("api-version")
		gotForm = form
		writeRefreshToken(t, w)
	})

	tok, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io", "fake-access", "my-tenant", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != fakeRefreshToken {
		t.Errorf("refresh token = %q, want %q", tok, fakeRefreshToken)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/oauth2/exchange" {
		t.Errorf("path = %q, want /oauth2/exchange", gotPath)
	}
	if gotAPIVersion != "2021-07-01" {
		t.Errorf("api-version = %q, want 2021-07-01", gotAPIVersion)
	}
	wantForm := map[string]string{
		"grant_type":   "access_token",
		"service":      "myregistry.azurecr.io",
		"access_token": "fake-access",
		"tenant":       "my-tenant",
	}
	for k, want := range wantForm {
		if got := gotForm.Get(k); got != want {
			t.Errorf("form field %s = %q, want %q", k, got, want)
		}
	}
}

// TestExchangeMissingRefreshToken pins the nil-refresh-token guard: the
// registry answering 200 without a refresh_token must be a clean error, not
// a panic.
func TestExchangeMissingRefreshToken(t *testing.T) {
	opts := newFakeExchange(t, func(_ url.Values, w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, `{}`); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	})

	_, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io", "fake-access", "", opts)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "no refresh token returned by registry") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestExchangeHTTPErrorSurfaces pins that registry error responses surface as
// *azcore.ResponseError carrying the status code.
func TestExchangeHTTPErrorSurfaces(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			opts := newFakeExchange(t, func(_ url.Values, w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if _, err := io.WriteString(w, `{"error":"denied"}`); err != nil {
					t.Errorf("failed to write response: %v", err)
				}
			})

			_, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io", "fake-access", "", opts)
			if err == nil {
				t.Fatal("expected error, got none")
			}
			var respErr *azcore.ResponseError
			if !errors.As(err, &respErr) {
				t.Fatalf("error is %T, want *azcore.ResponseError: %v", err, err)
			}
			if respErr.StatusCode != status {
				t.Errorf("status code = %d, want %d", respErr.StatusCode, status)
			}
		})
	}
}

// TestExchangeRetries pins that transient statuses are retried within the
// configured retry policy. Retry delays are shrunk so the test stays fast.
func TestExchangeRetries(t *testing.T) {
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var attempts int
			opts := newFakeExchange(t, func(_ url.Values, w http.ResponseWriter, _ *http.Request) {
				attempts++
				if attempts < 3 {
					w.WriteHeader(status)
					return
				}
				writeRefreshToken(t, w)
			})
			opts.Retry = policy.RetryOptions{
				MaxRetries:    3,
				RetryDelay:    time.Millisecond,
				MaxRetryDelay: 5 * time.Millisecond,
			}

			tok, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io", "fake-access", "", opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tok != fakeRefreshToken {
				t.Errorf("refresh token = %q, want %q", tok, fakeRefreshToken)
			}
			if attempts != 3 {
				t.Errorf("attempts = %d, want 3 (initial + 2 retries)", attempts)
			}
		})
	}
}

// TestExchangeDeadlineRespected proves the caller's deadline bounds the
// exchange: a hanging registry must surface a deadline error promptly rather
// than hang.
func TestExchangeDeadlineRespected(t *testing.T) {
	opts := newFakeExchange(t, func(_ url.Values, _ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // hang until the request is abandoned
	})
	opts.Retry = policy.RetryOptions{MaxRetries: 0}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := GetRegistryRefreshTokenFromAADExchange(ctx, "myregistry.azurecr.io", "fake-access", "", opts)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want a context.DeadlineExceeded", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("exchange took %v despite a 50ms deadline", elapsed)
	}
}

// TestExchangeContextCancelled proves caller cancellation propagates.
func TestExchangeContextCancelled(t *testing.T) {
	opts := newFakeExchange(t, func(_ url.Values, w http.ResponseWriter, _ *http.Request) {
		writeRefreshToken(t, w)
	})
	opts.Retry = policy.RetryOptions{MaxRetries: 0}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := GetRegistryRefreshTokenFromAADExchange(ctx, "myregistry.azurecr.io", "fake-access", "", opts)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want a context.Canceled", err)
	}
}

// TestExchangeInvalidEndpoint covers the toEndpoint error branch through the
// public entry point.
func TestExchangeInvalidEndpoint(t *testing.T) {
	_, err := GetRegistryRefreshTokenFromAADExchange(t.Context(), "myregistry.azurecr.io/%zz", "fake-access", "", nil)
	if err == nil {
		t.Fatal("expected error, got none")
	}
	if !strings.Contains(err.Error(), "invalid registry endpoint") {
		t.Errorf("unexpected error: %v", err)
	}
}
