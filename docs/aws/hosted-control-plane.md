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
[HMC ManagedCluster manifest
generation](#hmc-managed-cluster-manifest-generation) section.

If using the `aws-standalone-cp` template to deploy a hosted cluster it is
recommended to use a `t3.large` or larger instance type as the `hmc-controller`
and other provider controllers will need a large amount of resources to run.

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


## HMC ManagedCluster manifest

With all the collected data your `ManagedCluster` manifest will look similar to this:

```yaml
    apiVersion: hmc.mirantis.com/v1alpha1
    kind: ManagedCluster
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

## HMC ManagedCluster manifest generation

Grab the following `ManagedCluster` manifest template and save it to a file named `managedcluster.yaml.tpl`:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: aws-hosted
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

Then run the following command to create the `managedcluster.yaml`:

```
kubectl get awscluster cluster -o go-template="$(cat managedcluster.yaml.tpl)" > managedcluster.yaml
```
## Deployment Tips
* Ensure HMC templates and the controller image are somewhere public and
  fetchable.
* For installing the HMC charts and templates from a custom repository, load
  the `kubeconfig` from the cluster and run the commands:

```
KUBECONFIG=kubeconfig IMG="ghcr.io/mirantis/hmc/controller-ci:v0.0.1-179-ga5bdf29" REGISTRY_REPO="oci://ghcr.io/mirantis/hmc/charts-ci" make dev-apply
KUBECONFIG=kubeconfig make dev-templates
```
* The infrastructure will need to manually be marked `Ready` to get the
  `MachineDeployment` to scale up.  You can patch the `AWSCluster` kind using
  the command:

```
KUBECONFIG=kubeconfig kubectl patch AWSCluster <hosted-cluster-name> --type=merge --subresource status --patch 'status: {ready: true}' -n hmc-system
```

For additional information on why this is required [click here](https://docs.k0smotron.io/stable/capi-aws/#:~:text=As%20we%20are%20using%20self%2Dmanaged%20infrastructure%20we%20need%20to%20manually%20mark%20the%20infrastructure%20ready.%20This%20can%20be%20accomplished%20using%20the%20following%20command).


