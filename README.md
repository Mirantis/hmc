# Mirantis Hybrid Cloud Platform

## Installation

### TLDR

> kubectl apply -f <https://github.com/Mirantis/hmc/releases/download/v0.0.1/install.yaml>

or install using `helm`

> helm install hmc oci://ghcr.io/mirantis/hmc/charts/hmc --version v0.0.1 -n hmc-system --create-namespace

Then follow the [Deploy a managed cluster](#deploy-a-managed-cluster) guide to create a managed cluster.

> Note: The HMC installation using Kubernetes manifests does not allow customization of the deployment.
> To apply a custom HMC configuration, install HMC using the Helm chart.

### Development guide

See [Install HMC for development purposes](docs/dev.md#hmc-installation-for-development).

### Software Prerequisites

Mirantis Hybrid Container Cloud requires the following:

1. Existing management cluster (minimum required kubernetes version 1.28.0).
1. `kubectl` CLI installed locally.

Optionally, the following CLIs may be helpful:

1. `helm` (required only when installing HMC using `helm`).
1. `clusterctl` (to handle the lifecycle of the managed clusters).

### Providers configuration

Follow the instruction to configure providers. Currently supported providers:

* [AWS](docs/aws/main.md#prepare-the-aws-infra-provider)
* [Azure](docs/azure/main.md)

### Install

```bash
export KUBECONFIG=<path-to-management-kubeconfig>

helm install hmc oci://ghcr.io/mirantis/hmc/charts/hmc --version <hmc-version> -n hmc-system --create-namespace
```

See [HMC configuration options](templates/hmc/values.yaml).

#### Extended Management configuration

By default, the Hybrid Container Cloud is being deployed with the following configuration:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Management
metadata:
  name: hmc
spec:
  core:
    capi:
      template: cluster-api
    hmc:
      template: hmc
  providers:
  - template: k0smotron
  - config:
      configSecret:
       name: aws-variables
    template: cluster-api-provider-aws
```

There are two options to override the default management configuration of HMC:

1. Update the `Management` object after the HMC installation using `kubectl`:

    `kubectl --kubeconfig <path-to-management-kubeconfig> edit management`

1. Deploy HMC skipping the default `Management` object creation and provide your own `Management`
configuration:

   * Create `management.yaml` file and configure core components and providers.
   See [Management API](api/v1alpha1/management_types.go).

   * Specify `--create-management=false` controller argument and install HMC:

    If installing using `helm` add the following parameter to the `helm install` command:

    `--set="controller.createManagement=false"`

   * Create `hmc` `Management` object after HMC installation:

    `kubectl --kubeconfig <path-to-management-kubeconfig> create -f management.yaml`

## Deploy a managed cluster

To deploy a managed cluster:

1. Select the `Template` you want to use for the deployment. To list all available templates, run:

    ```bash
    export KUBECONFIG=<path-to-management-kubeconfig>

    kubectl get template -n hmc-system -o go-template='{{ range .items }}{{ if eq .status.type "deployment" }}{{ .metadata.name }}{{"\n"}}{{ end }}{{ end }}'
    ```

    For details about the `Template system` in HMC, see [Templates system](docs/templates/main.md#templates-system).

    If you want to deploy hostded control plate template, make sure to check additional notes on [Hosted control plane](docs/aws/hosted-control-plane.md).

1. Create the file with the `ManagedCluster` configuration:

    > Substitute the parameters enclosed in angle brackets with the corresponding values.\
    > Enable the `dryRun` flag if required. For details, see [Dry run](#dry-run).

    ```yaml
    apiVersion: hmc.mirantis.com/v1alpha1
    kind: ManagedCluster
    metadata:
      name: <cluster-name>
      namespace: <cluster-namespace>
    spec:
      template: <template-name>
      dryRun: <true/false>
      config:
        <cluster-configuration>
    ```

1. Create the `ManagedCluster` object:

    `kubectl create -f managedcluster.yaml`

1. Check the status of the newly created `ManagedCluster` object:

    `kubectl -n <managedcluster-namespace> get managedcluster <managedcluster-name> -o=yaml`

1. Wait for infrastructure to be provisioned and the cluster to be deployed (the provisioning starts only when
`spec.dryRun` is disabled):

    `kubectl -n <managedcluster-namespace> get cluster <managedcluster-name> -o=yaml`

    > You may also watch the process with the `clusterctl describe` command (requires the `clusterctl` CLI to be installed):
    >
    > ```bash
    > clusterctl describe cluster <managedcluster-name> -n <managedcluster-namespace> --show-conditions all
    > ```

1. Retrieve the `kubeconfig` of your managed cluster:

    ```bash
    kubectl get secret -n hmc-system <managedcluster-name>-kubeconfig -o=jsonpath={.data.value} | base64 -d > kubeconfig
    ```

### Dry run

HMC `ManagedCluster` supports two modes: with and without (default) `dryRun`.

If no configuration (`spec.config`) provided, the `ManagedCluster` object will be populated with defaults
(default configuration can be found in the corresponding `Template` status) and automatically marked as `dryRun`.

Here is an example of the `ManagedCluster` object with default configuration:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: <cluster-name>
  namespace: <cluster-namespace>
spec:
  config:
    clusterNetwork:
      pods:
        cidrBlocks:
        - 10.244.0.0/16
      services:
        cidrBlocks:
        - 10.96.0.0/12
    controlPlane:
      amiID: ""
      iamInstanceProfile: control-plane.cluster-api-provider-aws.sigs.k8s.io
      instanceType: ""
    controlPlaneNumber: 3
    k0s:
      version: v1.27.2+k0s.0
    publicIP: false
    region: ""
    sshKeyName: ""
    worker:
      amiID: ""
      iamInstanceProfile: nodes.cluster-api-provider-aws.sigs.k8s.io
      instanceType: ""
    workersNumber: 2
  template: aws-standalone-cp
  dryRun: true
```

After you adjust your configuration and ensure that it passes validation (`TemplateReady` condition
from `status.conditions`), remove the `spec.dryRun` flag to proceed with the deployment.

Here is an example of a `ManagedCluster` object that passed the validation:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: aws-standalone
  namespace: aws
spec:
  template: aws-standalone-cp
  config:
    region: us-east-2
    publicIP: true
    controlPlaneNumber: 1
    workersNumber: 1
    controlPlane:
      amiID: ami-02f3416038bdb17fb
      instanceType: t3.small
    worker:
      amiID: ami-02f3416038bdb17fb
      instanceType: t3.small
  status:
    conditions:
    - lastTransitionTime: "2024-07-22T09:25:49Z"
      message: Template is valid
      reason: Succeeded
      status: "True"
      type: TemplateReady
    - lastTransitionTime: "2024-07-22T09:25:49Z"
      message: Helm chart is valid
      reason: Succeeded
      status: "True"
      type: HelmChartReady
    - lastTransitionTime: "2024-07-22T09:25:49Z"
      message: ManagedCluster is ready
      reason: Succeeded
      status: "True"
      type: Ready
    observedGeneration: 1
```

## Cleanup

1. Remove the Management object:
  
    `kubectl delete management.hmc hmc`

    > Note: make sure you have no HMC ManagedCluster objects left in the cluster prior to Management deletion

1. Remove the `hmc` Helm release:

    `helm uninstall hmc -n hmc-system`

1. Remove the `hmc-system` namespace:

    `kubectl delete ns hmc-system`
