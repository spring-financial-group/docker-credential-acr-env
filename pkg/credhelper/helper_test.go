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

import "testing"

// TestIsACRRegistry tests our URL validation logic - the core of what this helper does
func TestIsACRRegistry(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		// Valid ACR registries
		{"myregistry.azurecr.io", true},
		{"myregistry.azurecr.cn", true},
		{"myregistry.azurecr.de", true},
		{"myregistry.azurecr.us", true},
		{"mcr.microsoft.com", true},

		// Not ACR
		{"myregistry.azurecr.me", false},
		{"docker.io", false},
		{"gcr.io", false},
		{"localhost:5000", false},
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
