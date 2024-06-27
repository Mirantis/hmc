# Mirantis Hybrid Cloud Platform

## Getting Started

Below is the example on how to deploy an HMC managed cluster on AWS with k0s. 
The kind cluster acts as management in this example.

### Prerequisites

#### Install `clusterawsadm`

1. Download the latest release of `clusterawsadm` binary:

Linux:
```
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/download/v0.0.0/clusterawsadm-linux-amd64 -o clusterawsadm
```

macOS:
```
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/download/v0.0.0/clusterawsadm-darwin-amd64 -o clusterawsadm

```
or if your Mac has an M1 CPU (”Apple Silicon”):
```
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-aws/releases/download/v0.0.0/clusterawsadm-darwin-arm64 -o clusterawsadm
```

2. Make it executable, move to a directory present in your PATH and check version:

```
chmod +x clusterawsadm
sudo mv clusterawsadm /usr/local/bin
clusterawsadm version
```

#### AWS IAM setup

Before launching a cluster, it's crucial to set up your AWS infrastructure provider:

> Note. Skip steps below if you've already configured IAM policy for your account

1. In order to use clusterawsadm you must have an administrative user in an AWS account. Once you have that 
administrator user you need to set your environment variables:

```
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=<admin-user-access-key>
export AWS_SECRET_ACCESS_KEY=<admin-user-secret-access-key>
export AWS_SESSION_TOKEN=<session-token> # Optional. If you are using Multi-Factor Auth.
```

2. After these are set run this command to create IAM cloud formation stack:

```
clusterawsadm bootstrap iam create-cloudformation-stack
```

#### Configure AWS credentials for the bootstrap

1. Ensure AWS user has enough permissions to deploy cluster. Ensure that these policies were attached to the AWS user:

* `control-plane.cluster-api-provider-aws.sigs.k8s.io`
* `controllers.cluster-api-provider-aws.sigs.k8s.io`
* `nodes.cluster-api-provider-aws.sigs.k8s.io`

2. Retrieve access key and export it as environment variable:

```
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=<your-access-key>
export AWS_SECRET_ACCESS_KEY=<your-secret-access-key>
export AWS_SESSION_TOKEN=<session-token> # Optional. If you are using Multi-Factor Auth.
```

3. Create the base64 encoded credentials using `clusterawsadm`. This command uses your environment variables and
encodes them in a value to be stored in a Kubernetes Secret.

```
export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)
```

#### Deploy HMC

1. Clone `Mirantis/hmc` repository

Example:
```
git clone https://github.com/Mirantis/hmc.git && cd hmc
```

2. Configure your cluster parameters in `config/dev/deployment.yaml`:

   * Configure the `name` of the deployment
   * Change `amiID` and `instanceType` for control plane and worker machines
   * Specify the number of control plane and worker machines, etc

4. Run `make dev-apply` to deploy and configure management cluster

5. Wait a couple of minutes for management components to be up and running

6. Run `make dev-aws-apply` to deploy managed cluster on AWS with default configuration

7. Wait for infrastructure to be provisioned and the cluster to be deployed. You may watch the process with the
`clusterctl describe` command. Example:

```
export KUBECONFIG=~/.kube/config

clusterctl describe cluster <deployment-name> -n hmc-system --show-conditions all
```

8. Retrieve the `kubeconfig` of your managed cluster:

```
kubectl --kubeconfig ~/.kube/config get secret -n hmc-system <deployment-name>-kubeconfig -o=jsonpath={.data.value} | base64 -d > kubeconfig
```
