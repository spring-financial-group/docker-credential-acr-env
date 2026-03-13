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

// Package azcloud maps environment configuration to Azure cloud
// definitions, restoring the AZURE_ENVIRONMENT support that go-autorest's
// auth.GetSettingsFromEnvironment provided.
package azcloud

import (
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
)

// FromEnvironment maps AZURE_ENVIRONMENT to a cloud.Configuration, accepting
// the environment names go-autorest used (case-insensitively)
//
//	An unset or empty value selects the public cloud
func FromEnvironment() (cloud.Configuration, error) {
	raw := os.Getenv("AZURE_ENVIRONMENT")
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "", "AZUREPUBLICCLOUD":
		return cloud.AzurePublic, nil
	case "AZURECHINACLOUD":
		return cloud.AzureChina, nil
	case "AZUREUSGOVERNMENTCLOUD":
		return cloud.AzureGovernment, nil
	default:
		return cloud.Configuration{}, fmt.Errorf(
			"unknown Azure environment %q (supported: AzurePublicCloud, AzureChinaCloud, AzureUSGovernmentCloud)",
			raw)
	}
}

// ACRTokenScope returns the AAD token scope for Azure Container Registry in the given cloud
func ACRTokenScope(cfg cloud.Configuration) (string, error) {
	svc, ok := cfg.Services[azcontainerregistry.ServiceName]
	if !ok || svc.Audience == "" {
		return "", fmt.Errorf("no Azure Container Registry audience configured for cloud %q", cfg.ActiveDirectoryAuthorityHost)
	}
	return svc.Audience + "/.default", nil
}
