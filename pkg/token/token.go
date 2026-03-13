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
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	envFederatedToken     = "AZURE_FEDERATED_TOKEN"
	envFederatedTokenFile = "AZURE_FEDERATED_TOKEN_FILE"
	envClientID           = "AZURE_CLIENT_ID"
	envTenantID           = "AZURE_TENANT_ID"
)

type AADAccessTokenResponse struct {
	AccessToken string
	TenantID    string
}

type TokenProvider interface {
	GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// NewDefaultTokenProvider builds the credential chain for the given cloud.
//
// azidentity's DefaultAzureCredential only honours AZURE_FEDERATED_TOKEN_FILE;
// an inline AZURE_FEDERATED_TOKEN (which master supported and the README
// documents) is restored here via a client-assertion credential. The JWT is
// passed through a callback rather than a temp file, so it never touches
// disk. AZURE_FEDERATED_TOKEN_FILE, when also set, takes precedence and is
// left to the default chain's workload-identity credential.
func NewDefaultTokenProvider(cfg cloud.Configuration) (TokenProvider, error) {
	if _, ok := os.LookupEnv(envFederatedToken); ok {
		if _, fileSet := os.LookupEnv(envFederatedTokenFile); !fileSet {
			return newInlineAssertionProvider(cfg, nil)
		}
	}
	return azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
		ClientOptions: azcore.ClientOptions{Cloud: cfg},
	})
}

// newInlineAssertionProvider builds a client-assertion credential from the
// inline AZURE_FEDERATED_TOKEN. transport is nil in production; tests inject
// one to reroute AAD traffic to a fake.
func newInlineAssertionProvider(cfg cloud.Configuration, transport policy.Transporter) (TokenProvider, error) {
	assertion := os.Getenv(envFederatedToken)
	clientID := os.Getenv(envClientID)
	if clientID == "" {
		return nil, fmt.Errorf("%s is set but %s is not", envFederatedToken, envClientID)
	}
	tenantID := os.Getenv(envTenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("%s is set but %s is not", envFederatedToken, envTenantID)
	}
	return azidentity.NewClientAssertionCredential(tenantID, clientID,
		func(context.Context) (string, error) { return assertion, nil },
		&azidentity.ClientAssertionCredentialOptions{
			ClientOptions: azcore.ClientOptions{Cloud: cfg, Transport: transport},
		})
}

// GetAADAccessToken requests an AAD access token for the given scope.
func GetAADAccessToken(ctx context.Context, tp TokenProvider, scope string) (AADAccessTokenResponse, error) {
	tok, err := tp.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return AADAccessTokenResponse{}, fmt.Errorf("failed to fetch AAD token: %w", err)
	}

	return AADAccessTokenResponse{
		AccessToken: tok.Token,
		TenantID:    os.Getenv(envTenantID),
	}, nil
}
