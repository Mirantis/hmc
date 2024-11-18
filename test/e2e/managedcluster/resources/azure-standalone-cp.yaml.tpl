apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
  namespace: ${NAMESPACE}
spec:
  template: azure-standalone-cp-0-0-3
  credential: ${AZURE_CLUSTER_IDENTITY}-cred
  config:
    controlPlaneNumber: 1
    workersNumber: 1
    location: "${AZURE_REGION}"
    subscriptionID: "${AZURE_SUBSCRIPTION_ID}"
    controlPlane:
      vmSize: Standard_A4_v2
    worker:
      vmSize: Standard_A4_v2
    credential: ${AZURE_CLUSTER_IDENTITY}-cred
    clusterIdentity:
      name: ${AZURE_CLUSTER_IDENTITY}
      namespace: ${NAMESPACE}
    tenantID: "${AZURE_TENANT_ID}"
    clientID: "${AZURE_CLIENT_ID}"
    clientSecret: "${AZURE_CLIENT_SECRET}"
