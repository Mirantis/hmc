#!/bin/bash
# Copyright 2024
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script will remove all resources affiliated with the AWS CCM, such as
# ELB or CSI driver resources that can not be filtered by cloud-nuke.
# It should be ran after running cloud-nuke to remove any remaining resources.
if [ -z $CLUSTER_NAME ]; then
    echo "CLUSTER_NAME must be set"
    exit 1
fi

if [ -z $YQ ]; then
    echo "YQ must be set to the path of the yq binary"
    echo "Use 'make dev-aws-nuke' instead of running this script directly"
    exit 1
fi

if [ -z $AWSCLI ]; then
    echo "AWSCLI must be set to the path of the AWS CLI"
    echo "Use 'make dev-aws-nuke' instead of running this script directly"
    exit 1
fi

if [ "$1" == "elb" ]; then
    echo "Checking for ELB with '$CLUSTER_NAME' tag"
    for LOADBALANCER in $($AWSCLI elb describe-load-balancers --output yaml | $YQ '.LoadBalancerDescriptions[].LoadBalancerName');
    do
        echo "Checking ELB: $LOADBALANCER for tag"
        DESCRIBE_TAGS=$($AWSCLI elb describe-tags --load-balancer-names $LOADBALANCER --output yaml | $YQ '.TagDescriptions[]' | grep $CLUSTER_NAME)
        if [ ! -z "${DESCRIBE_TAGS}" ]; then
            echo "Deleting ELB: $LOADBALANCER"
            $AWSCLI elb delete-load-balancer --load-balancer-name $LOADBALANCER
        fi
    done
fi

if [ "$1" == "ebs" ]; then
    echo "Checking for EBS Volumes with '$CLUSTER_NAME' within the 'kubernetes.io/created-for/pvc/name' tag"
    for VOLUME in $($AWSCLI  ec2 describe-volumes --output yaml | $YQ '.Volumes[].VolumeId');
    do
        echo "Checking EBS Volume: $VOLUME for $CLUSTER_NAME claim"
        DESCRIBE_VOLUMES=$($AWSCLI ec2 describe-volumes \
            --volume-id $VOLUME \
            --output yaml | $YQ '.Volumes | to_entries[] | .value.Tags[] | select(.Key == "kubernetes.io/created-for/pvc/name")' | grep $CLUSTER_NAME)
        if [ ! -z "${DESCRIBE_VOLUMES}" ]; then
            echo "Deleting EBS Volume: $VOLUME"
            $AWSCLI ec2 delete-volume --volume-id $VOLUME
        fi
    done
fi
