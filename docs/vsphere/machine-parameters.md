# vSphere machine parameters

## SSH

Currently SSH configuration on vSphere expects that user is already created
during template creation. Because of that you must pass username along with SSH
public key to configure SSH access.


SSH public key can be passed to `.spec.config.ssh.publicKey` (in case of
hosted CP) parameter or `.spec.config.controlPlane.ssh.publicKey` and
`.spec.config.worker.ssh.publicKey` parameters (in case of standalone CP) of the
`ManagedCluster` object.

SSH public key must be passed literally as a string.

Username can be passed to `.spec.config.controlPlane.ssh.user`,
`.spec.config.worker.ssh.user` or `.spec.config.ssh.user` depending on you
deployment model.

## VM resources

The following parameters are used to define VM resources:

| Parameter         | Example | Description                                                          |
|-------------------|---------|----------------------------------------------------------------------|
| `.rootVolumeSize` | `50`    | Root volume size in GB (can't be less than one defined in the image) |
| `.cpus`           | `2`     | Number of CPUs                                                       |
| `.memory`         | `4096`  | Memory size in MB                                                    |

The resource parameters are the same for hosted and standalone CP deployments,
but they are positioned differently in the spec, which means that they're going to:

- `.spec.config` in case of hosted CP deployment.
- `.spec.config.controlPlane` in in case of standalone CP for control plane
  nodes.
- `.spec.config.worker` in in case of standalone CP for worker nodes.

## VM Image and network

To provide image template path and network path the following parameters must be
used:

| Parameter     | Example           | Description         |
|---------------|-------------------|---------------------|
| `.vmTemplate` | `/DC/vm/template` | Image template path |
| `.network`    | `/DC/network/Net` | Network path        |

As with resource parameters the position of these parameters in the
`ManagedCluster` depends on deployment type and these parameters are used in:

- `.spec.config` in case of hosted CP deployment.
- `.spec.config.controlPlane` in in case of standalone CP for control plane
  nodes.
- `.spec.config.worker` in in case of standalone CP for worker nodes.
