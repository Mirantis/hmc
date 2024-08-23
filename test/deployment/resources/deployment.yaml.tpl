apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
    name: ${DEPLOYMENT_NAME}
spec:
    template: ${TEMPLATE_NAME}
    config:
        region: ${AWS_REGION}
        publicIP: ${PUBLIC_IP:=true}
        controlPlaneNumber: ${CONTROL_PLANE_NUMBER:=1}
        workersNumber: ${WORKERS_NUMBER:=1}
        controlPlane:
            amiID: ${AMI_ID}
            instanceType: ${INSTANCE_TYPE:=t3.small}
        worker:
            amiID: ${AMI_ID}
            instanceType: ${INSTANCE_TYPE:=t3.small}

