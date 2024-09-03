apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
  name: ${DEPLOYMENT_NAME}
  namespace: ${NAMESPACE:=default}
spec:
  template: aws-standalone-cp
  config:
    region: ${AWS_REGION}
    publicIP: ${PUBLIC_IP:=true}
    controlPlaneNumber: ${CONTROL_PLANE_NUMBER:=1}
    workersNumber: ${WORKERS_NUMBER:=1}
    controlPlane:
      instanceType: ${INSTANCE_TYPE:=t3.small}
    worker:
      instanceType: ${INSTANCE_TYPE:=t3.small}


