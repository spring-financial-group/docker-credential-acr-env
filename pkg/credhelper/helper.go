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
package credhelper

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/spring-financial-group/docker-credential-acr-env/pkg/azcloud"
	"github.com/spring-financial-group/docker-credential-acr-env/pkg/registry"
	"github.com/spring-financial-group/docker-credential-acr-env/pkg/token"
)

var acrSuffixes = []string{".azurecr.io", ".azurecr.cn", ".azurecr.us"}

const (
	mcrHostname   = "mcr.microsoft.com"
	tokenUsername = "<token>"

	// AAD token acquisition and the registry token exchange share this deadline
	defaultTimeout = 30 * time.Second
)

type ACRCredHelper struct {
	tokenProvider   token.TokenProvider
	cloud           cloud.Configuration
	exchangeOptions *azcontainerregistry.AuthenticationClientOptions
}

func NewACRCredentialsHelper() (credentials.Helper, error) {
	cfg, err := azcloud.FromEnvironment()
	if err != nil {
		return nil, err
	}
	tp, err := token.NewDefaultTokenProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}
	return &ACRCredHelper{tokenProvider: tp, cloud: cfg}, nil
}

// isACRRegistry reports whether input names an Azure Container Registry
func isACRRegistry(input string) bool {
	serverURL, err := url.Parse("https://" + input)
	if err != nil {
		return false
	}
	host := strings.ToLower(serverURL.Hostname())
	if host == mcrHostname {
		return true
	}
	for _, suffix := range acrSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func (a ACRCredHelper) Get(serverURL string) (string, string, error) {
	if !isACRRegistry(serverURL) {
		return "", "", errors.New("serverURL does not refer to Azure Container Registry")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	scope, err := azcloud.ACRTokenScope(a.cloud)
	if err != nil {
		return "", "", err
	}

	tok, err := token.GetAADAccessToken(ctx, a.tokenProvider, scope)
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire AAD token: %w", err)
	}

	exchangeOpts := a.exchangeOptions
	if exchangeOpts == nil {
		exchangeOpts = exchangeClientOptions(a.cloud)
	}

	refreshToken, err := registry.GetRegistryRefreshTokenFromAADExchange(ctx, serverURL, tok.AccessToken, tok.TenantID, exchangeOpts)
	if err != nil {
		return "", "", fmt.Errorf("failed to acquire refresh token: %w", err)
	}

	return tokenUsername, refreshToken, nil
}

// exchangeClientOptions returns production options for the registry exchange client
func exchangeClientOptions(cfg cloud.Configuration) *azcontainerregistry.AuthenticationClientOptions {
	return &azcontainerregistry.AuthenticationClientOptions{
		ClientOptions: azcore.ClientOptions{
			Cloud: cfg,
			Retry: policy.RetryOptions{
				MaxRetries:    2,
				MaxRetryDelay: 5 * time.Second,
			},
		},
	}
}

func (a ACRCredHelper) Add(_ *credentials.Credentials) error {
	return errors.New("add is unimplemented")
}

func (a ACRCredHelper) Delete(_ string) error {
	return errors.New("delete is unimplemented")
}

func (a ACRCredHelper) List() (map[string]string, error) {
	return nil, errors.New("list is unimplemented")
}
