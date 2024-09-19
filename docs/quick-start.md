# Mirantis Hybrid Cloud Platform

## Installation

### TLDR

    kubectl apply -f https://github.com/Mirantis/hmc/releases/download/v0.0.1/install.yaml

or install using `helm`

    helm install hmc oci://ghcr.io/mirantis/hmc/charts/hmc --version v0.0.1 -n hmc-system --create-namespace


> Note: The HMC installation using Kubernetes manifests does not allow customization of the deployment. To apply a custom HMC configuration, install HMC using the Helm chart.
> deployment. If the custom HMC configuration should be applied, install HMC using
> the Helm chart.