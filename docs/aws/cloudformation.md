# AWS IAM setup

Before launching a cluster on AWS, it's crucial to set up your AWS infrastructure provider:

> Note. Skip steps below if you've already configured IAM policy for your account

1. In order to use clusterawsadm you must have an administrative user in an AWS account. Once you have that
   administrator user you need to set your environment variables:

```
export AWS_REGION=<aws-region>
export AWS_ACCESS_KEY_ID=<admin-user-access-key>
export AWS_SECRET_ACCESS_KEY=<admin-user-secret-access-key>
export AWS_SESSION_TOKEN=<session-token> # Optional. If you are using Multi-Factor Auth.
```

2. After these are set run this command to create IAM cloud formation stack:

```
clusterawsadm bootstrap iam create-cloudformation-stack
```
