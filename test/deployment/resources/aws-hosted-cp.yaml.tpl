apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
  name: ${DEPLOYMENT_NAME}
spec:
  template: aws-hosted-cp
  config:
    vpcID: ${AWS_VPC_ID}
    region: ${AWS_REGION}
    publicIP: ${PUBLIC_IP:=true}
    subnets:
      - id: ${AWS_SUBNET_ID}
        availabilityZone: ${AWS_SUBNET_AVAILABILITY_ZONE}
    instanceType: ${INSTANCE_TYPE:=t3.medium}
    securityGroupIDs:
      - ${AWS_SG_ID}
