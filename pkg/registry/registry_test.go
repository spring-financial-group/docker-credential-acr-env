package registry

import (
	"testing"
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
