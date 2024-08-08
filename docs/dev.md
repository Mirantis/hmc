# HMC installation for development

Below is the example on how to install HMC for development purposes and create
a managed cluster on AWS with k0s for testing. The kind cluster acts as management in this example.

## Prerequisites

### Clone HMC repository

```
git clone https://github.com/Mirantis/hmc.git && cd hmc
```

### Install required CLIs

Run:

```
make cli-install
```

### AWS Provider Setup

Follow the instruction to configure AWS Provider: [AWS Provider Setup](aws/main.md#prepare-the-aws-infra-provider)

## Deploy HMC

1. Configure your cluster parameters in `config/dev/deployment.yaml`:

    * Configure the `name` of the deployment
    * Change `amiID` and `instanceType` for control plane and worker machines
    * Specify the number of control plane and worker machines, etc

2. Run `make dev-apply` to deploy and configure management cluster

3. Wait a couple of minutes for management components to be up and running

4. Run `make dev-aws-apply` to deploy managed cluster on AWS with default configuration

5. Wait for infrastructure to be provisioned and the cluster to be deployed. You may watch the process with the
   `./bin/clusterctl describe` command. Example:

```
export KUBECONFIG=~/.kube/config

./bin/clusterctl describe cluster <deployment-name> -n hmc-system --show-conditions all
```

> [!NOTE]
> If you encounter any errors in the output of `clusterctl describe cluster` inspect the logs of the
> `cluster-api-provider-aws-controller-manager` with:
> ```
> kubectl logs -n hmc-system deploy/cluster-api-provider-aws-controller-manager
> ```
> This may help identify any potential issues with deployment of the AWS infrastructure.

6. Retrieve the `kubeconfig` of your managed cluster:

```
kubectl --kubeconfig ~/.kube/config get secret -n hmc-system <deployment-name>-kubeconfig -o=jsonpath={.data.value} | base64 -d > kubeconfig
```
