apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
  namespace: ${NAMESPACE}
spec:
  template: azure-hosted-cp
  config:
    controlPlaneNumber: 1
    workersNumber: 1
    location: "westus"
    subscriptionID: "${AZURE_SUBSCRIPTION_ID}"
    controlPlane:
      vmSize: Standard_A4_v2
    worker:
      vmSize: Standard_A4_v2
    clusterIdentity:
      name: azure-cluster-identity
      namespace: ${NAMESPACE}
    tenantID: "${AZURE_TENANT_ID}"
    clientID: "${AZURE_CLIENT_ID}"
    clientSecret: "${AZURE_CLIENT_SECRET}"
