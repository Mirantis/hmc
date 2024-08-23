apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
    name: ${DEPLOYMENT_NAME}
spec:
    config:
        controlPlane:
            amiID: ${AMI_ID}
            instanceType: ${INSTANCE_TYPE}
        controlPlaneNumber: ${CONTROL_PLANE_NUMBER}
        publicIP: ${PUBLIC_IP}
        region: ${AWS_REGION}
        worker:
            amiID: ${AMI_ID}
            instanceType: ${INSTANCE_TYPE}
        workersNumber: ${WORKERS_NUMBER}
    template: ${TEMPLATE_NAME}
