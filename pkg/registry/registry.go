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
package registry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
)

// GetRegistryRefreshTokenFromAADExchange exchanges an AAD token for an ACR refresh token.
func GetRegistryRefreshTokenFromAADExchange(serverURL string, accessToken string, tenantID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeOut)
	defer cancel()

	endpoint, err := toEndpoint(serverURL)
	if err != nil {
		return "", fmt.Errorf("invalid registry endpoint: %w", err)
	}

	client, err := azcontainerregistry.NewAuthenticationClient(endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth client: %w", err)
	}

	resp, err := client.ExchangeAADAccessTokenForACRRefreshToken(
		ctx,
		azcontainerregistry.PostContentSchemaGrantTypeAccessToken,
		serverURL,
		&azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions{
			AccessToken: &accessToken,
			Tenant:      to.Ptr(tenantID),
		})
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}

	if resp.ACRRefreshToken.RefreshToken == nil {
		return "", errors.New("no refresh token returned by registry")
	}

	return *resp.ACRRefreshToken.RefreshToken, nil
}

// toEndpoint ensures the server URL has a scheme and returns the endpoint.
func toEndpoint(serverURL string) (string, error) {
	s := serverURL
	if !strings.HasPrefix(s, secureScheme) {
		s = secureScheme + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	return u.Scheme + "://" + u.Host, nil
}
