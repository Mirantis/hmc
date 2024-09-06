# Azure cluster parameters

## Prerequisites

- Azure CLI installed
- `az login` command executed

## Cluster Identity

To provide credentials for CAPI Azure provider (CAPZ) the `AzureClusterIdentity`
resource must be created. This should be done before provisioning any clusters.


To create the `AzureClusterIdentity` you should first get the desired
`SubscriptionID` by executing `az account list -o table` which will return list
of subscriptions available to user.

Then you need to create service principal which will be used by CAPZ to interact
with Azure API. To do so you need to execute the following command:

```bash
az ad sp create-for-rbac --role contributor --scopes="/subscriptions/<Subscription ID>"
```

The command will return json with the credentials for the service principal which
will look like this:

```json
{
	"appId": "29a3a125-7848-4ce6-9be9-a4b3eecca0ff",
	"displayName": "azure-cli",
	"password": "u_RANDOMHASH",
	"tenant": "2f10bc28-959b-481f-b094-eb043a87570a",
}
```

*Note: make sure to save this credentials and treat them like passwords.*

With the data from the json you can now create the `AzureClusterIdentity` object
and it's secret.

The objects created with the data above can look something like this:

**Secret**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: az-cluster-identity-secret
  namespace: hmc-system
stringData:
  clientSecret: u_RANDOMHASH
type: Opaque
```

**AzureClusterIdentity**:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  labels:
    clusterctl.cluster.x-k8s.io/move-hierarchy: "true"
  name: az-cluster-identity
  namespace: hmc-system
spec:
  allowedNamespaces: {}
  clientID: 29a3a125-7848-4ce6-9be9-a4b3eecca0ff
  clientSecret:
    name: az-cluster-identity-secret
    namespace: hmc-system
  tenantID: 2f10bc28-959b-481f-b094-eb043a87570a
  type: ServicePrincipal
```

These objects then should be referenced in the `ManagedCluster` object in the
`.spec.config.clusterIdentity` field.

Subscription ID which was used to create service principal should be the
same that will be used in the `.spec.config.subscriptionID` field of the
`ManagedCluster` object.
