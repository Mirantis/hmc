# Hosted control plane (k0smotron) deployment

## Prerequisites

-   Management Kubernetes cluster (v1.28+) deployed on AWS with HMC installed on it
-   Default storage class configured on the management cluster
-   VPC id for the worker nodes
-   Subnet ID which will be used along with AZ information
-   AMI id which will be used to deploy worker nodes

Keep in mind that all control plane components for all managed clusters will
reside in the management cluster.

## Networking

The networking resources in AWS which are needed for a managed cluster can be
reused with a management cluster.

If you deployed your AWS Kubernetes cluster using Cluster API Provider AWS (CAPA)
you can obtain all the necessary data with the commands below or use the
template found below in the
[HMC Deployment manifest generation](#hmc-deployment-manifest-generation) section.

**VPC ID**

```bash
    kubectl get awscluster <cluster name> -o go-template='{{.spec.network.vpc.id}}'
```

**Subnet ID**

```bash
    kubectl get awscluster <cluster name> -o go-template='{{(index .spec.network.subnets 0).resourceID}}'
```

**Availability zone**

```bash
    kubectl get awscluster <cluster name> -o go-template='{{(index .spec.network.subnets 0).availabilityZone}}'
```

**Security group**
```bash
    kubectl get awscluster <cluster name> -o go-template='{{.status.networkStatus.securityGroups.node.id}}'
```

**AMI id**

```bash
    kubectl get awsmachinetemplate <cluster name>-worker-mt -o go-template='{{.spec.template.spec.ami.id}}'
```

If you want to use different VPCs/regions for your management or managed clusters
you should setup additional connectivity rules like [VPC peering](https://docs.aws.amazon.com/whitepapers/latest/building-scalable-secure-multi-vpc-network-infrastructure/vpc-peering.html).


## HMC Deployment manifest

With all the collected data your `Deployment` manifest will look similar to this:

```yaml
    apiVersion: hmc.mirantis.com/v1alpha1
    kind: Deployment
    metadata:
      name: aws-hosted-cp
    spec:
      template: aws-hosted-cp
      config:
        vpcID: vpc-0a000000000000000
        region: us-west-1
        publicIP: true
        subnets:
          - id: subnet-0aaaaaaaaaaaaaaaa
            availabilityZone: us-west-1b
        amiID: ami-0bfffffffffffffff
        instanceType: t3.medium
        securityGroupIDs:
          - sg-0e000000000000000
```

> [!NOTE]
> In this example we're using the `us-west-1` region, but you should use the region of your VPC.

## HMC Deployment manifest generation

Grab the following `Deployment` manifest template and save it to a file named `deployment.yaml.tpl`:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
  name: aws-hosted-cp
spec:
  template: aws-hosted-cp
  config:
    vpcID: "{{.spec.network.vpc.id}}"
    region: "{{.spec.region}}"
    subnets:
      - id: "{{(index .spec.network.subnets 0).resourceID}}"
        availabilityZone: "{{(index .spec.network.subnets 0).availabilityZone}}"
    amiID: ami-0bf2d31c356e4cb25
    instanceType: t3.medium
    securityGroupIDs:
      - "{{.status.networkStatus.securityGroups.node.id}}"
```

Then run the following command to create the `deployment.yaml`:

```
kubectl get awscluster cluster -o go-template="$(cat deployment.yaml.tpl)" > deployment.yaml
```
