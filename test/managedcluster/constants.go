// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package managedcluster

const (
	// Common
	EnvVarManagedClusterName       = "MANAGED_CLUSTER_NAME"
	EnvVarHostedManagedClusterName = "HOSTED_MANAGED_CLUSTER_NAME"
	EnvVarInstallBeachHeadServices = "INSTALL_BEACH_HEAD_SERVICES"
	EnvVarControlPlaneNumber       = "CONTROL_PLANE_NUMBER"
	EnvVarWorkerNumber             = "WORKER_NUMBER"
	EnvVarNamespace                = "NAMESPACE"
	// EnvVarNoCleanup disables After* cleanup in provider specs to allow for
	// debugging of test failures.
	EnvVarNoCleanup = "NO_CLEANUP"

	// AWS
	EnvVarAWSVPCID                  = "AWS_VPC_ID"
	EnvVarAWSSubnetID               = "AWS_SUBNET_ID"
	EnvVarAWSSubnetAvailabilityZone = "AWS_SUBNET_AVAILABILITY_ZONE"
	EnvVarAWSInstanceType           = "AWS_INSTANCE_TYPE"
	EnvVarAWSSecurityGroupID        = "AWS_SG_ID"
	EnvVarPublicIP                  = "AWS_PUBLIC_IP"
	AWSCredentialsSecretName        = "aws-variables"
)
