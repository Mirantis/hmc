# Azure provider

## Prerequisites

1. `kubectl` CLI installed locally.
2. `az` CLI utility installed (part of `azure-cli`).
3. Azure account with the required resource providers registered.

## Resource providers registration

The following resource providers should be registered in your Azure account:

- `Microsoft.Compute`
- `Microsoft.Network`
- `Microsoft.ContainerService`
- `Microsoft.ManagedIdentity`
- `Microsoft.Authorization`

You can follow the [official documentation guide](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-providers-and-types)
to register the providers.

## Azure cluster parameters

Follow the [Azure cluster parameters](cluster-parameters.md) guide to setup
mandatory parameters for Azure clusters.

## Azure machine parameters

Follow the [Azure machine parameters](machine-parameters.md) guide if you want to
setup/modify the default machine parameters.

## Azure hosted control plane

Follow the [Hosted control plane](hosted-control-plane.md) guide to deploy
hosted control plane cluster on Azure.
