# Azure machine parameters

## SSH

SSH public key can be passed to `.spec.config.sshPublicKey` (in case of hosted CP)
parameter or `.spec.config.controlPlane.sshPublicKey` and
`.spec.config.worker.sshPublicKey` parameters (in case of standalone CP)
of the `ManagedCluster` object.

It should be encoded in **base64** format.

## VM size

Azure supports various VM sizes which can be retrieved with the following
command:

```bash
az vm list-sizes --location "<location>" -o table
```

Then desired VM size could be passed to the:

- `.spec.config.vmSize` - for hosted CP deployment.
- `.spec.config.controlPlane.vmSize` - for control plane nodes in the standalone
  deployment.
- `.spec.config.worker.vmSize` - for worker nodes in the standalone deployment.

*Example: Standard_A4_v2*

## Root Volume size

Root volume size of the VM (in GB) can be changed through the following
parameters:

- `.spec.config.rootVolumeSize` - for hosted CP deployment.
- `.spec.config.controlPlane.rootVolumeSize` - for control plane nodes in the
  standalone deployment.
- `.spec.config.worker.rootVolumeSize` - for worker nodes in the standalone
  deployment.

*Default value: 30*

Please note that this value can't be less than size of the root volume which
defined in your image.

## VM Image

You can define the image which will be used for you machine using the following
parameters:

- `.spec.config.image` - for hosted CP deployment.
- `.spec.config.controlPlane.image` - for control plane nodes in the standalone
  deployment.
- `.spec.config.worker.image` - for worker nodes in the standalone deployment.

There are multiple self-excluding ways to define the image source (e.g. Azure
Compute Gallery, Azure Marketplace, etc.).

Detailed information regarding image can be found in [CAPZ documentation](https://capz.sigs.k8s.io/self-managed/custom-images)

By default, the latest official CAPZ Ubuntu based image is used.
