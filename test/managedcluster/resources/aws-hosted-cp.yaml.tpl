apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${HOSTED_MANAGED_CLUSTER_NAME}
spec:
  template: aws-hosted-cp
  config:
    vpcID: ${AWS_VPC_ID}
    region: ${AWS_REGION}
    subnets:
      - id: ${AWS_SUBNET_ID}
        availabilityZone: ${AWS_SUBNET_AVAILABILITY_ZONE}
    instanceType: ${AWS_INSTANCE_TYPE:=t3.medium}
    securityGroupIDs:
      - ${AWS_SG_ID}
