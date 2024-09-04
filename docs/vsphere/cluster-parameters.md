# vSphere cluster parameters

## Prerequisites

- vSphere provider [prerequisites](main.md#prerequisites) are complete.

## Cluster Identity

To provide credentials for CAPI vSphere provider (CAPV) the
`VSphereClusterIdentity` resource must be created. This should be done before
provisioning any clusters.

To create cluster identity you'll only need username and password for your
vSphere instance.

The example of the objects can be found below:

**Secret**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vsphere-cluster-identity-secret
  namespace: hmc-system
stringData:
  username: user
  password: Passw0rd
```

**VsphereClusterIdentity**:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: VSphereClusterIdentity
metadata:
  name: vsphere-cluster-identity
  namespace: hmc-system
spec:
  secretName: vsphere-cluster-identity-secret
  allowedNamespaces:
    selector:
      matchLabels: {}
```

These objects then should be referenced in the `ManagedCluster` object in the
`.spec.config.clusterIdentity` field.


## ManagedCluster parameters

To deploy managed cluster a number of parameters should be passed to the
`ManagedCluster` object.

### Parameter list

The following is the list of vSphere specific parameters, which are _required_
for successful cluster creation.

| Parameter                           | Example                               | Description                        |
|-------------------------------------|---------------------------------------|------------------------------------|
| `.spec.config.vsphere.server`       | `vcenter.example.com`                 | Address of the vSphere server      |
| `.spec.config.vsphere.thumbprint`   | `"00:00:00"`                          | Certificate thumbprint             |
| `.spec.config.vsphere.datacenter`   | `DC`                                  | Datacenter name                    |
| `.spec.config.vsphere.datastore`    | `/DC/datastore/DS`                    | Datastore path                     |
| `.spec.config.vsphere.resourcePool` | `/DC/host/vCluster/Resources/ResPool` | Resource pool path                 |
| `.spec.config.vsphere.folder`       | `/DC/vm/example`                      | vSphere folder path                |
| `.spec.config.vsphere.username`     | `user`                                | Username for your vSphere instance |
| `.spec.config.vsphere.password`     | `password`                            | Password for your vSphere instance |

_You can check [machine parameters](machine-parameters.md) for machine specific
parameters._

To obtain vSphere certificate thumbprint you can use the following command:

```bash
curl -sw %{certs} https://vcenter.example.com | openssl x509 -sha256 -fingerprint -noout | awk -F '=' '{print $2}'
```

Username and password currently must be passed once more in the `ManagedCluster`
object to provide proper authentication for CCM and CSI driver.

## Example of ManagedCluster CR

With all above parameters provided your `ManagedCluster` can look like this:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: cluster-1
spec:
  template: vsphere-standalone-cp
  config:
    clusterIdentity:
      name: vsphere-cluster-identity
    vsphere:
      server: vcenter.example.com
      thumbprint: "00:00:00"
      datacenter: "DC"
      datastore: "/DC/datastore/DC"
      resourcePool: "/DC/host/vCluster/Resources/ResPool"
      folder: "/DC/vm/example"
      username: "user"
      password: "Passw0rd"
    controlPlaneEndpointIP: "172.16.0.10"

    controlPlane:
      ssh:
        user: ubuntu
        publicKey: |
          ssh-rsa AAA...
      rootVolumeSize: 50
      cpus: 2
      memory: 4096
      vmTemplate: "/DC/vm/template"
      network: "/DC/network/Net"

    worker:
      ssh:
        user: ubuntu
        publicKey: |
          ssh-rsa AAA...
      rootVolumeSize: 50
      cpus: 2
      memory: 4096
      vmTemplate: "/DC/vm/template"
      network: "/DC/network/Net"
```
