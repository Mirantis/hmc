# AWS Credentials configuration

1. Ensure AWS user has enough permissions to deploy cluster. Ensure that these policies were attached to the AWS user:

* `control-plane.cluster-api-provider-aws.sigs.k8s.io`
* `controllers.cluster-api-provider-aws.sigs.k8s.io`
* `nodes.cluster-api-provider-aws.sigs.k8s.io`

2. Retrieve access key and export it as environment variable:

```
export AWS_REGION=<aws-region>
export AWS_ACCESS_KEY_ID=<your-access-key>
export AWS_SECRET_ACCESS_KEY=<your-secret-access-key>
export AWS_SESSION_TOKEN=<session-token> # Optional. If you are using Multi-Factor Auth.
```

3. Create the base64 encoded credentials using `clusterawsadm`:

```
export AWS_B64ENCODED_CREDENTIALS=$(clusterawsadm bootstrap credentials encode-as-profile)
```

4. Create the secret with AWS variables:

> By default, HMC fetches the AWS variables configuration from the `aws-variables` secret in the `hmc-system`
> namespace. If you want to change the name of the secret you should overwrite the configuration of the cluster 
> API provider AWS in the HMC Management object. \
> For details, see: [Extended Management Configuration](../../README.md#extended-management-configuration)

> You can also provide additional configuration variables, but the `AWS_B64ENCODED_CREDENTIALS` parameter is required.

```
kubectl create secret generic aws-variables -n hmc-system --from-literal AWS_B64ENCODED_CREDENTIALS="$AWS_B64ENCODED_CREDENTIALS"
```
