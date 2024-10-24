apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
spec:
  template: aws-eks-0-0-2
  credential: ${AWS_CLUSTER_IDENTITY}-cred
  config:
    region: ${AWS_REGION}
    workersNumber: ${WORKERS_NUMBER:=1}
    worker:
      instanceType: ${AWS_INSTANCE_TYPE:=t3.small}
