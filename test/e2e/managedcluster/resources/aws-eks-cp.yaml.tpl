apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}-eks
spec:
  template: aws-eks-0-0-2
  credential: ${AWS_CLUSTER_IDENTITY}-cred
  config:
    region: ${AWS_REGION}
    workersNumber: ${WORKERS_NUMBER:=1}
    clusterIdentity:
      name:  ${AWS_CLUSTER_IDENTITY}-cred
      namespace: ${NAMESPACE}
    publicIP: ${AWS_PUBLIC_IP:=true}
    worker:
      instanceType: ${AWS_INSTANCE_TYPE:=t3.small}
