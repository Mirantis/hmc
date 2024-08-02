# Mirantis Hybrid Cloud Platform

## Installation

### TLDR

> kubectl apply -f https://github.com/Mirantis/hmc/releases/download/v0.0.1/install.yaml

or install using `helm`

> helm install hmc oci://ghcr.io/mirantis/hmc/charts/hmc --version v0.0.1 -n hmc-system --create-namespace

Then follow the [Deploy a managed cluster](#deploy-a-managed-cluster) guide to create a managed cluster.

### Development guide

See [Install HMC for development purposes](docs/dev.md#hmc-installation-for-development).

### Software Prerequisites

Mirantis Hybrid Container Cloud requires the following:

1. Existing management cluster (minimum required kubernetes version 1.28.0).
2. `kubectl` CLI installed locally.

Optionally, the following CLIs may be helpful:

1. `helm` (required only when installing HMC using `helm`).
2. `clusterctl` (to handle the lifecycle of the managed clusters). 

### Providers configuration

Follow the instruction to configure providers. Currently supported providers:
* [AWS](docs/aws/main.md#prepare-the-aws-infra-provider)

### Installation

```
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
  namespace: hmc-system
spec:
  core:
    capi:
      template: cluster-api
    hmc:
      template: hmc
  providers:
  - template: k0smotron
  - config:
      credentialsSecretName: aws-credentials
    template: cluster-api-provider-aws
```

There are two options to override the default management configuration of HMC:

1. Update the `Management` object after the HMC installation using `kubectl`:

    `kubectl --kubeconfig <path-to-management-kubeconfig> -n hmc-system edit management`

2. Deploy HMC skipping the default `Management` object creation and provide your own `Management`
configuration:

   * Create `management.yaml` file and configure core components and providers.
   See [Management API](api/v1alpha1/management_types.go).

   * Specify `--create-management=false` controllerManager argument and install HMC:

    If installing using `helm` add the following parameter to the `helm install` command:

    `--set="controllerManager.manager.args={--create-management=false}"`

   * Create `hmc-system/hmc` `Management` object after HMC installation:

    `kubectl --kubeconfig <path-to-management-kubeconfig> -n hmc-system create -f management.yaml`

## Deploy a managed cluster

To deploy a managed cluster:

1. Select the `Template` you want to use for the deployment. To list all available templates, run:

```bash
export KUBECONFIG=<path-to-management-kubeconfig>

kubectl get template -n hmc-system -o go-template='{{ range .items }}{{ if eq .status.type "deployment" }}{{ .metadata.name }}{{ printf "\n" }}{{ end }}{{ end }}'
```

For details about the `Template system` in HMC, see [Templates system](docs/templates/main.md#templates-system).

If you want to deploy hostded control plate template, make sure to check additional notes on [Hosted control plane](docs/aws/hosted-control-plane.md).

2. Create the file with the `Deployment` configuration:

> Substitute the parameters enclosed in angle brackets with the corresponding values.\
> Enable the `dryRun` flag if required. For details, see [Dry run](#dry-run).

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
  name: <cluster-name>
  namespace: <cluster-namespace>
spec:
  template: <template-name>
  dryRun: <true/false>
  config:
    <cluster-configuration>
```

3. Create the `Deployment` object:

`kubectl create -f deployment.yaml`

4. Check the status of the newly created `Deployment` object:

`kubectl -n <deployment-namespace> get deployment.hmc <deployment-name> -o=yaml`

5. Wait for infrastructure to be provisioned and the cluster to be deployed (the provisioning starts only when
`spec.dryRun` is disabled):

   `kubectl -n <deployment-namespace> get cluster <deployment-name> -o=yaml`

> You may also watch the process with the `clusterctl describe` command (requires the `clusterctl` CLI to be installed):
> ```
> clusterctl describe cluster <deployment-name> -n <deployment-namespace> --show-conditions all
> ```

6. Retrieve the `kubeconfig` of your managed cluster:

```
kubectl get secret -n hmc-system <deployment-name>-kubeconfig -o=jsonpath={.data.value} | base64 -d > kubeconfig
```

### Dry run

HMC `Deployment` supports two modes: with and without (default) `dryRun`.

If no configuration (`spec.config`) provided, the `Deployment` object will be populated with defaults
(default configuration can be found in the corresponding `Template` status) and automatically marked as `dryRun`.

Here is an example of the `Deployment` object with default configuration:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
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

Here is an example of a `Deployment` object that passed the validation:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
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
      message: Deployment is ready
      reason: Succeeded
      status: "True"
      type: Ready
    observedGeneration: 1
```
