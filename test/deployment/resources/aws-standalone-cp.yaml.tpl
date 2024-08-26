apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
  name: ${DEPLOYMENT_NAME}
spec:
  template: aws-standalone-cp
  config:
    region: ${AWS_REGION}
    publicIP: ${PUBLIC_IP:=true}
    controlPlaneNumber: ${CONTROL_PLANE_NUMBER:=1}
    workersNumber: ${WORKERS_NUMBER:=1}
    controlPlane:
      amiID: ${AWS_AMI_ID}
      instanceType: ${INSTANCE_TYPE:=t3.small}
    worker:
      amiID: ${AWS_AMI_ID}
      instanceType: ${INSTANCE_TYPE:=t3.small}


