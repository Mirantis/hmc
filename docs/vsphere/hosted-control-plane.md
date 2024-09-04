# Hosted control plane (k0smotron) deployment

## Prerequisites

- Management Kubernetes cluster (v1.28+) deployed on vSphere with HMC installed
  on it

Keep in mind that all control plane components for all managed clusters will
reside in the management cluster.


## ManagedCluster manifest

Hosted CP template has mostly identical parameters with standalone CP, you can
check them in the [cluster parameters](cluster-parameters.md) and the
[machine parameters](machine-parameters.md) sections.

**Important note on control plane endpoint IP**

Since vSphere provider requires that user will provide control plane endpoint IP
before deploying the cluster you should make sure that this IP will be the same
that will be assigned to the k0smotron LB service. Thus you must provide control
plane endpoint IP to the k0smotron service via annotation which is accepted by
your LB provider (in the following example `kube-vip` annotation is used)

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: ManagedCluster
metadata:
  name: cluster-1
spec:
  template: vsphere-hosted-cp
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

    ssh:
      user: ubuntu
      publicKey: |
        ssh-rsa AAA...
    rootVolumeSize: 50
    cpus: 2
    memory: 4096
    vmTemplate: "/DC/vm/template"
    network: "/DC/network/Net"

    k0smotron:
      service:
        annotations:
          kube-vip.io/loadbalancerIPs: "172.16.0.10"
```
