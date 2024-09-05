apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: ${MANAGED_CLUSTER_NAME}
spec:
  template: aws-standalone-cp
  config:
    region: ${AWS_REGION}
    publicIP: ${AWS_PUBLIC_IP:=true}
    controlPlaneNumber: ${CONTROL_PLANE_NUMBER:=1}
    workersNumber: ${WORKERS_NUMBER:=1}
    controlPlane:
      instanceType: ${AWS_INSTANCE_TYPE:=t3.small}
    worker:
      instanceType: ${AWS_INSTANCE_TYPE:=t3.small}
    installBeachHeadServices: ${INSTALL_BEACH_HEAD_SERVICES:=true}


