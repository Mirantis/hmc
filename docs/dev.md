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

### Azure Provider Setup

Follow the instruction on how to configure [Azure Provider](azure/main.md).

Additionally to deploy dev cluster on Azure the following env variables should
be set before running deployment:

- `AZURE_SUBSCRIPTION_ID` - Subscription ID
- `AZURE_TENANT_ID` - Service principal tenant ID
- `AZURE_CLIENT_ID` - Service principal App ID
- `AZURE_CLIENT_SECRET` - Service principal password

More detailed description of these parameters can be found
[here](azure/cluster-parameters.md).

## Deploy HMC

Default provider which will be used to deploy cluster is AWS, if you want to use
another provider change `DEV_PROVIDER` variable with the name of provider before
running make (e.g. `export DEV_PROVIDER=azure`).

1. Configure your cluster parameters in provider specific file
   (for example `config/dev/aws-managedcluster.yaml` in case of AWS):

    * Configure the `name` of the ManagedCluster
    * Change instance type or size for control plane and worker machines
    * Specify the number of control plane and worker machines, etc

2. Run `make dev-apply` to deploy and configure management cluster.

3. Wait a couple of minutes for management components to be up and running.

4. Apply credentials for your provider by executing `make dev-creds-apply`.

5. Run `make dev-provider-apply` to deploy managed cluster on provider of your
   choice with default configuration.

6. Wait for infrastructure to be provisioned and the cluster to be deployed. You
   may watch the process with the `./bin/clusterctl describe` command. Example:

```
export KUBECONFIG=~/.kube/config

./bin/clusterctl describe cluster <managedcluster-name> -n hmc-system --show-conditions all
```

> [!NOTE]
> If you encounter any errors in the output of `clusterctl describe cluster` inspect the logs of the
> `capa-controller-manager` with:
> ```
> kubectl logs -n hmc-system deploy/capa-controller-manager
> ```
> This may help identify any potential issues with deployment of the AWS infrastructure.

7. Retrieve the `kubeconfig` of your managed cluster:

```
kubectl --kubeconfig ~/.kube/config get secret -n hmc-system <managedcluster-name>-kubeconfig -o=jsonpath={.data.value} | base64 -d > kubeconfig
```
