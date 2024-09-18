apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
  namespace: ${NAMESPACE}
spec:
  template: azure-hosted-cp
  config:
    location: "westus"
    subscriptionID: "${AZURE_SUBSCRIPTION_ID}"
    vmSize: Standard_A4_v2
    clusterIdentity:
      name: azure-cluster-identity
      namespace: hmc-system
    resourceGroup: "${AZURE_RESOURCE_GROUP}"
    network:
      vnetName: "${AZURE_VM_NET_NAME}"
      nodeSubnetName: "${AZURE_NODE_SUBNET}"
      routeTableName: "${AZURE_ROUTE_TABLE}"
      securityGroupName: "${AZURE_SECURITY_GROUP}"
    tenantID: "${AZURE_TENANT_ID}"
    clientID: "${AZURE_CLIENT_ID}"
    clientSecret: "${AZURE_CLIENT_SECRET}"
