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

4. Create the secret with AWS credentials in the `hmc-system` namespace:

```
kubectl create secret generic aws-credentials -n hmc-system --from-literal credentials="$(echo $AWS_B64ENCODED_CREDENTIALS | base64 -d)"
```
