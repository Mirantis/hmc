# Hosted control plane (k0smotron) deployment

## Prerequisites

-   Management Kubernetes cluster (v1.28+) deployed on Azure with HMC installed
    on it
-   Default storage class configured on the management cluster

Keep in mind that all control plane components for all managed clusters will
reside in the management cluster.

## Pre-existing resources

Certain resources will not be created automatically in a hosted control plane
scenario thus they should be created in advance and provided in the `ManagedCluster`
object. You can reuse these resources with management cluster as described
below.

If you deployed your Azure Kubernetes cluster using Cluster API Provider Azure
(CAPZ) you can obtain all the necessary data with the commands below:

**Location**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{.spec.location}}'
```

**Subscription ID**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{.spec.subscriptionID}}'
```

**Resource group**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{.spec.resourceGroup}}'
```

**vnet name**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{.spec.networkSpec.vnet.name}}'
```

**Subnet name**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{(index .spec.networkSpec.subnets 1).name}}'
```

**Route table name**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{(index .spec.networkSpec.subnets 1).routeTable.name}}'
```

**Security group name**

```bash
kubectl get azurecluster <cluster name> -o go-template='{{(index .spec.networkSpec.subnets 1).securityGroup.name}}'
```



## HMC ManagedCluster manifest

With all the collected data your `ManagedCluster` manifest will look similar to this:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: azure-hosted-cp
spec:
  template: azure-hosted-cp
  config:
    location: "westus"
    subscriptionID: ceb131c7-a917-439f-8e19-cd59fe247e03
    vmSize: Standard_A4_v2
    clusterIdentity:
      name: az-cluster-identity
      namespace: hmc-system
    resourceGroup: mgmt-cluster
    network:
      vnetName: mgmt-cluster-vnet
      nodeSubnetName: mgmt-cluster-node-subnet
      routeTableName: mgmt-cluster-node-routetable
      securityGroupName: mgmt-cluster-node-nsg
    tenantID: 7db9e0f2-c88a-4116-a373-9c8b6cc9d5eb
    clientID: 471f65fa-ddee-40b4-90ae-da1a8a114ee1
    clientSecret: "u_RANDOM"
```

To simplify creation of the ManagedCluster object you can use the template below:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: azure-hosted-cp
spec:
  template: azure-hosted-cp
  config:
    location: "{{.spec.location}}"
    subscriptionID: "{{.spec.subscriptionID}}"
    vmSize: Standard_A4_v2
    clusterIdentity:
      name: az-cluster-identity
      namespace: hmc-system
    resourceGroup: "{{.spec.resourceGroup}}"
    network:
      vnetName: "{{.spec.networkSpec.vnet.name}}"
      nodeSubnetName: "{{(index .spec.networkSpec.subnets 1).name}}"
      routeTableName: "{{(index .spec.networkSpec.subnets 1).routeTable.name}}"
      securityGroupName: "{{(index .spec.networkSpec.subnets 1).securityGroup.name}}"
    tenantID: 7db9e0f2-c88a-4116-a373-9c8b6cc9d5eb
    clientID: 471f65fa-ddee-40b4-90ae-da1a8a114ee1
    clientSecret: "u_RANDOM"
```

Then you can render it using the command:

```bash
kubectl get azurecluster <management cluster name> -o go-template="$(cat template.yaml)"
```

## Cluster creation

After applying `ManagedCluster` object you require to manually set the status of the
`AzureCluster` object due to current limitations (see k0sproject/k0smotron#668).

To do so you need to execute the following command:

```bash
kubectl patch azurecluster <cluster name> --type=merge --subresource status --patch 'status: {ready: true}'
```

## Important notes on the cluster deletion

Because of the aforementioned limitation you also need to make manual steps in
order to properly delete cluster.

Before removing the cluster make sure to place custom finalizer onto
`AzureCluster` object. This is needed to prevent it from being deleted instantly
which will cause cluster deletion to stuck indefinitely.

To place finalizer you can execute the following command:

```bash
kubectl patch azurecluster <cluster name> --type=merge --patch 'metadata: {finalizers: [manual]}'
```

When finalizer is placed you can remove the `ManagedCluster` as usual. Check that
all `AzureMachines` objects are deleted successfully and remove finalizer you've
placed to finish cluster deletion.

In case if have orphaned `AzureMachines` left you have to delete finalizers on
them manually after making sure that no VMs are present in Azure.

*Note: since Azure admission prohibits orphaned objects mutation you'll have to
disable it by deleting it's `mutatingwebhookconfiguration`*
