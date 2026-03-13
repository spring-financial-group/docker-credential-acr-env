# ACR Docker Credential Helper

The ACR docker credential helper is an alternative to the existing file store based ACR helper 
located [here](https://github.com/Azure/acr-docker-credential-helper) which relies on `az` command
line and is not optimised for use in CI environments. Primary use case for this helper is for use
with kaniko and other tools running in CI scenarios wishing to push to Azure Container Registry

## How it works

The credential helper is built on the Azure SDK for Go (`azidentity`) and sources its configuration
from well-known Azure environmental information. It attempts to authenticate firstly via client
credentials grant if the following environment config is present

```
AZURE_CLIENT_ID=<clientID>
AZURE_CLIENT_SECRET=<clientSecret>
AZURE_TENANT_ID=<tenantId>
```

Client certificate authentication (`AZURE_CLIENT_CERTIFICATE_PATH`) is also supported through the
same chain.

If the details needed for the client credential grant are not set it will try to 
find a [federated OIDC JWT](https://learn.microsoft.com/en-us/graph/api/resources/federatedidentitycredentials-overview?view=graph-rest-1.0) 
in the enviroment. To use this set the following values in the enviroment.

```
AZURE_CLIENT_ID=<clientID>
AZURE_FEDERATED_TOKEN=<federatedJWT>
AZURE_TENANT_ID=<tenantId>
```

`AZURE_FEDERATED_TOKEN_FILE` may be used instead of the inline `AZURE_FEDERATED_TOKEN` and takes
precedence when both are set.

If you use federated OIDC with [Azure Workload Identity](https://github.com/Azure/azure-workload-identity) you don't
have to set any ENVs as they will get injected automatically.

If the above are not set then authentication falls back through the remainder of the
`DefaultAzureCredential` chain: managed identity (which works in various Azure contexts such as
App Service and Azure Kubernetes Service), then the Azure CLI (`az`), Azure Developer CLI (`azd`),
and Azure PowerShell if they are installed and logged in.

## Sovereign clouds

`AZURE_ENVIRONMENT` selects the Azure cloud to authenticate against, using the same names
go-autorest accepted (case-insensitive):

- `AzurePublicCloud` (the default when unset)
- `AzureChinaCloud`
- `AzureUSGovernmentCloud`

An unknown value is an error rather than a silent fallback to the public cloud. Alternatively,
azidentity's `AZURE_AUTHORITY_HOST` can be used to point at an authority directly.
`AZURE_AD_RESOURCE` (honoured by go-autorest) is no longer read; the ACR token audience is
derived from the selected cloud. Azure Germany is retired and `.azurecr.de` registries are no
longer accepted.

## Notes for importers of the `pkg/` packages

The `azidentity` refactor changed public function signatures:

- `registry.GetRegistryRefreshTokenFromAADExchange` now takes a caller-owned `context.Context`
  (which should carry the deadline for the whole operation) and a
  `*azcontainerregistry.AuthenticationClientOptions` (nil for production defaults).
- `token.NewDefaultTokenProvider` takes a `cloud.Configuration` (see `pkg/azcloud.FromEnvironment`).
- `token.GetAADAccessToken` takes the token scope explicitly (see `pkg/azcloud.ACRTokenScope`).
