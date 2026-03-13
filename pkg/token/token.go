/*
Copyright © 2020 Chris Mellard chris.mellard@icloud.com

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
package token

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// AADAccessTokenResponse wraps an AAD access token and tenant info
type AADAccessTokenResponse struct {
	AccessToken string
	TenantID    string
}

// GetAADAccessToken retrieves an access token for ACR using the default chain.
func GetAADAccessToken(ctx context.Context) (AADAccessTokenResponse, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return AADAccessTokenResponse{}, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://containerregistry.azure.net/.default"},
	})
	if err != nil {
		return AADAccessTokenResponse{}, fmt.Errorf("failed to fetch AAD token: %w", err)
	}

	return AADAccessTokenResponse{
		AccessToken: tok.Token,
		TenantID:    os.Getenv("AZURE_TENANT_ID"),
	}, nil
}
