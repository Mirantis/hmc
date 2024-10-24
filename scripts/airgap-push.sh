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
#
# This script can be used to help users re-tag and push images and Helm charts
# into a private registry for use when deploying HMC ManagedClusters into an
# air-gapped environment.  This script is packaged as part of the airgap bundle
# for convenience.

REPO=""
CHART_REPO=""
AIRGAP_BUNDLE=""
HELP=""

# Parse the options
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -h|--help)
            HELP="true"
            shift
            ;;
        -r|--image-repo)
            REPO="$2"
            shift
            shift
            ;;
        -c|--chart-repo)
            CHART_REPO="$2"
            shift
            shift
            ;;
        -u|--username)
            USERNAME="$2"
            shift
            shift
            ;;
        -p|--password)
            PASSWORD="$2"
            shift
            shift
            ;;
        -a|--airgap-bundle)
            AIRGAP_BUNDLE="$2"
            shift
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Print the help message
print_help() {
    echo "Usage:"
    echo "  airgap-push.sh [OPTIONS]"
    echo "Options:"
    echo "  -h, --help"
    echo "    Print this help message"
    echo "  -r, --image-repo (required)"
    echo "    The image repo to push the images to, if you do not wish to push the images to a specific repo, set this value to your registry URL"
    echo "  -c, --chart-repo (required)"
    echo "    The repository to push the Helm charts to, for OCI prefix with oci://"
    echo "  -u, --username (required)"
    echo "    The username to use for the registry"
    echo "  -p, --password (required)"
    echo "    The password to use for the registry"
    echo "  -a, --airgap-bundle (required)"
    echo "    The path to the airgap bundle"
}

if [ ! -z "$HELP" ]; then
    print_help
    exit 0
fi

if [ -z "$NEW_REPO" ]; then
    echo "The repository must be set"
    print_help
    exit 1
fi

if [ -z "$CHART_REPO" ]; then
    echo "The chart repository must be set"
    print_help
    exit 1
fi

if [ -z "$AIRGAP_BUNDLE" ]; then
    echo "The airgap bundle must be set"
    exit 1
else
    # Validate the airgap bundle
    if [ ! -d "$AIRGAP_BUNDLE" ]; then
        echo "The provided airgap bundle: $AIRGAP_BUNDLE does not exist"
        exit 1
    fi
fi

# Load the images into the local Docker daemon.
docker load -i $AIRGAP_BUNDLE

# Extract the repositories json file from the airgap bundle.
tar xf $AIRGAP_BUNDLE "repositories"

# Iterate over the images in the repositories json file and retag and push them
# to the given repository.
docker login $REPO -u $USERNAME -p $PASSWORD

for IMAGE in $(cat repositories | jq -r 'to_entries[] | .key'); do
    IMAGE_NAME=$(echo $IMAGE | grep -o '[^/]*$')
    OLD_IMAGE=$(docker images -a | grep $IMAGE | awk '{print $1":"$2}')
    TAG=$OLD_IMAGE | awk -F ":" '{print $2}'

    docker tag $OLD_IMAGE $REPO/$IMAGE_NAME:$TAG
    docker push $REPO/$IMAGE_NAME:$TAG
done

# Next, use Helm to push the charts to the given chart repository.
mkdir -p hmc_charts
tar xf $AIRGAP_BUNDLE "charts/extensions" -C hmc_charts/

helm registry login $CHART_REPO -u $USERNAME -p $PASSWORD

for CHART in $(ls hmc_charts/extensions); do
    helm push hmc_charts/extensions/$CHART $CHART_REPO
done
