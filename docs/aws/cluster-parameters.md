# AWS cluster parameters

## Software prerequisites

1. `clusterawsadm` CLI installed locally.
2. `AWS_B64ENCODED_CREDENTIALS` environment variable to be exported.
See [AWS credentials](credentials.md#aws-credentials-configuration) (p. 1-3)

## AWS AMI

Use `clusterawsadm` to get available AMIs to deploy managed cluster:

```bash
clusterawsadm ami list
```

For details, see [Pre-built Kubernetes AMIs](https://cluster-api-aws.sigs.k8s.io/topics/images/built-amis.html).

## SSH access to cluster nodes

To access the nodes using the SSH protocol, several things should be configured:

- An SSH key added in the region where you want to deploy the cluster
- Bastion host is enabled

### SSH keys

Only one SSH key is supported and it should be added in AWS prior to creating
the `Deployment` object. The name of the key should then be placed under `.spec.config.sshKeyName`.

The same SSH key will be used for all machines and a bastion host.

To enable bastion you should add `.spec.config.bastion.enabled` option in the
`Deployment` object to `true`.

Full list of the bastion configuration options could be fould in [CAPA docs](https://cluster-api-aws.sigs.k8s.io/crd/#infrastructure.cluster.x-k8s.io/v1beta1.Bastion).

The resulting `Deployment` can look like this:

```yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Deployment
metadata:
  name: cluster-1
spec:
  template: aws-standalone-cp
  config:
    sshKeyName: foobar
    bastion:
      enabled: true
...
```
