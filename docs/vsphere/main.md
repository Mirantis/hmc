# vSphere provider

## Prerequisites

1. `kubectl` CLI installed locally.
2. vSphere instance version `6.7.0` or higher.
3. vSphere account with appropriate privileges.
4. Image template.
5. vSphere network with DHCP enabled.

### Image template

You can use pre-buit image templates from [CAPV project](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere/blob/main/README.md#kubernetes-versions-with-published-ovas)
or build your own.

When building your own image make sure that vmware tools and cloud-init are
installed and properly configured.

You can follow [official open-vm-tools guide](https://docs.vmware.com/en/VMware-Tools/11.0.0/com.vmware.vsphere.vmwaretools.doc/GUID-C48E1F14-240D-4DD1-8D4C-25B6EBE4BB0F.html)
on how to correctly install vmware-tools.

When setting up cloud-init you can refer to [official docs](https://cloudinit.readthedocs.io/en/latest/index.html)
and specifically [vmware datasource docs](https://cloudinit.readthedocs.io/en/latest/reference/datasources/vmware.html)
for extended information regarding cloud-init on vSphere.

### vSphere network

When creating network make sure that it has DHCP service.

Also make sure that the part of your network is out of DHCP range (e.g. network
172.16.0.0/24 with DHCP range 172.16.0.100-172.16.0.254). This is needed to make
sure that LB services will not create any IP conflicts in the network.

### vSphere privileges

To function properly the user assigned to vSphere provider should be able to
manipulate vSphere resources. The following is the general overview of the
required privileges:

- `Virtual machine` - full permissions are required
- `Network` - `Assign network` is sufficient
- `Datastore` - it should be possible for user to manipulate virtual machine
  files and metadata

In addition to that specific CSI driver permissions are required see
[the official doc](https://docs.vmware.com/en/VMware-vSphere-Container-Storage-Plug-in/2.0/vmware-vsphere-csp-getting-started/GUID-0AB6E692-AA47-4B6A-8CEA-38B754E16567.html)
to get more information on CSI specific permissions.

## vSphere cluster parameters

Follow the [vSphere cluster parameters](cluster-parameters.md) guide to setup
mandatory parameters for vSphere clusters.

## vSphere machine parameters

Follow the [vSphere machine parameters](machine-parameters.md) guide if you want
to setup/modify the default machine parameters.

## vSphere hosted control plane

Follow the [Hosted control plane](hosted-control-plane.md) guide to deploy
hosted control plane cluster on vSphere.
