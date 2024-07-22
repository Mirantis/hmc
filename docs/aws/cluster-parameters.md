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
