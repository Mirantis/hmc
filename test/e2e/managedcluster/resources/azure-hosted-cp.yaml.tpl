apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
  namespace: ${NAMESPACE}
spec:
  template: azure-hosted-cp-0-0-3${BUILD_VERSION}
  credential: ${AZURE_CLUSTER_IDENTITY}-cred
  config:
    location: "${AZURE_REGION}"
    subscriptionID: "${AZURE_SUBSCRIPTION_ID}"
    vmSize: Standard_A4_v2
    clusterIdentity:
      name: ${AZURE_CLUSTER_IDENTITY}
      namespace: hmc-system
    resourceGroup: "${AZURE_RESOURCE_GROUP}"
    network:
      vnetName: "${AZURE_VM_NET_NAME}"
      nodeSubnetName: "${AZURE_NODE_SUBNET}"
      nodeRouteTableName: "${AZURE_NODE_ROUTE_TABLE}"
      nodeSecurityGroupName: "${AZURE_NODE_SECURITY_GROUP}"
      cpSubnetName: "${AZURE_CP_SUBNET}"
      cpRouteTableName: "${AZURE_CP_ROUTE_TABLE}"
      cpSecurityGroupName: "${AZURE_CP_SECURITY_GROUP}"
    tenantID: "${AZURE_TENANT_ID}"
    clientID: "${AZURE_CLIENT_ID}"
    clientSecret: "${AZURE_CLIENT_SECRET}"
