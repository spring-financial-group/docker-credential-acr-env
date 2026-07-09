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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type AADAccessTokenResponse struct {
	AccessToken string
	TenantID    string
}

type TokenProvider interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

func NewDefaultTokenProvider() (TokenProvider, error) {
	return azidentity.NewDefaultAzureCredential(nil)
}

func GetAADAccessToken(ctx context.Context, tp TokenProvider) (AADAccessTokenResponse, error) {
	tok, err := tp.GetToken(ctx, policy.TokenRequestOptions{
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
