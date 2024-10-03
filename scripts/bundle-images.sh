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
# bundle-images.sh bundles all of the images used across HMC into a single
# tarball.  This is useful for deploying HMC into air-gapped environments.
# It is recommended to use this script in conjunction with the Makefile target
# `make bundle-images` which will perform some additional steps outside of
# this scripts functionality.
# Usage: make bundle-images

if [ "$YQ" == "" ] ||
    [ "$IMG" == "" ] ||
    [ "$KUBECTL" == "" ] ||
    [ "$HELM" == "" ] ||
    [ "$KIND_CLUSTER_NAME" == "" ] ||
    [ "$NAMESPACE" == "" ] ||
    [ "$BUNDLE_TARBALL" == "" ] ||
    [ "$TEMPLATES_DIR" == "" ];
    then
        echo "This script should not be run directly.  Use 'make bundle-images' instead."
        exit 1
fi

LABEL_KEY="cluster.x-k8s.io/provider"
IMAGES_BUNDLED="$IMG"

echo -e "\nBundling images for HMC, this may take awhile...\n"

trap ctrl_c INT

function wait_for_deploy_exist() {
    local deployment_label=$1
    local max_wait_secs=300
    local interval_secs=5
    local start_time


    start_time=$(date +%s)

    echo "Verifying provider Deployment with label: \"$deployment_label\" exists in namespace: \"$NAMESPACE\"..."

    while true; do
        current_time=$(date +%s)
            if (( (current_time - start_time) > max_wait_secs )); then
                echo "Error: Waited for Deployment with label: \"$deployment_label\" in namespace: \"$NAMESPACE\" to exist for $max_wait_secs seconds and it still does not exist."
                return 1
            fi

        output=$($KUBECTL -n "$NAMESPACE" get deploy -l $deployment_label)

        if [[ output != "" ]]; then
            echo "Deployment in namespace: \"$NAMESPACE\" with label: \"$deployment_label\" exists."
            break
        else
            sleep $interval_secs
        fi
    done
}


function ctrl_c() {
        echo "Caught CTRL-C, exiting..."
        exit 1
}

echo -e "\nVerifying provider Deployments are ready...\n"

# Verify each provider we support has deployed so we can get the images used
# across the deployments.
for TEMPLATE in $(find $TEMPLATES_DIR -name 'provider.yaml');
do
    PROVIDER_YAML=$(grep -P '(?=.*Provider)(?=.*kind)' -A 2 $TEMPLATE | grep -v '\--')
    PROVIDER_KIND=$(echo -e "$PROVIDER_YAML" | $YQ e '.kind' -)
    PROVIDER_NAME=$(echo -e "$PROVIDER_YAML" | $YQ e '.metadata.name' -)
    PROVIDER_KIND_TOLOWER=$(echo ${PROVIDER_KIND,,})

    if [[ $PROVIDER_NAME == "" ]]; then
        echo "Error: Cannot determine provider Name from $TEMPLATE"
        exit 1
    fi

    if [[ $PROVIDER_KIND_TOLOWER == "" ]]; then
        echo "Error: Cannot determine provider Kind from $TEMPLATE"
        exit 1
    fi

    # controlplane is a special case which needs a hyphen.
    if [[ $PROVIDER_KIND_TOLOWER == "controlplane" ]]; then
        PROVIDER_KIND_TOLOWER="control-plane"
    fi

    # coreprovider does not have a provider prefix.
    if [[ $PROVIDER_KIND_TOLOWER == "coreprovider" ]]; then
        LABEL_VALUE=$(echo $PROVIDER_NAME)
    else
        LABEL_VALUE=$(echo $(echo $PROVIDER_KIND_TOLOWER | sed -e 's/provider//g')-$PROVIDER_NAME)
    fi

    wait_for_deploy_exist "$LABEL_KEY=$LABEL_VALUE"

    $KUBECTL wait --for condition=available --timeout=2m deploy -l $LABEL_KEY=$LABEL_VALUE -n $NAMESPACE
    if [[ $? -ne 0 ]]; then
        echo "Error: Cannot wait for Deployment: Deployment with $LABEL_KEY=$LABEL_VALUE label not found"
        exit 1
    fi
done


# Now that we know everything is deployed and ready, we can get all of images by
# execing into the KIND cluster.
CONTROL_PLANE=$($KUBECTL get nodes --no-headers -o custom-columns=":metadata.name")
if [[ $? -ne 0 ]] || [[ $CONTROL_PLANE == "" ]]; then
    echo "Error: Cannot get control plane node"
    exit 1
fi

echo -e "\nPulling images for HMC components...\n"

for IMAGE in $(docker exec -it $CONTROL_PLANE crictl images | sed 1,1d | awk '{print $1":"$2}' | grep -v 'kindest');
do
    if [[ $IMAGE == "" ]]; then
        echo "Error: Failed to get image from KIND cluster, image string should not be empty"
        exit 1
    fi

    if [[ $IMAGE == "docker.io/hmc/controller:latest" ]]; then
        # Don't try to pull and package the local controller image.
        continue
    fi

    docker pull $IMAGE
    if [[ $? -ne 0 ]]; then
        echo "Error: Failed to pull $IMAGE"
        exit 1
    fi

    IMAGES_BUNDLED="$IMAGES_BUNDLED $IMAGE"
done

echo -e "\nPulling images for HMC extensions...\n"

# Next, we need to build a list of images used by k0s extensions.  Walk the
# templates directory and extract the images used by the extensions.
for TEMPLATE in $(find $TEMPLATES_DIR -name 'k0s*.yaml');
do
    if [[ $TEMPLATE == *"k0smotron"* ]]; then
        EXTENSIONS_PATH=".spec.k0sConfig.spec.extensions.helm"
    else
        EXTENSIONS_PATH=".spec.k0sConfigSpec.k0s.spec.extensions.helm"
    fi

    REPOS=$(grep -vw "{{" $TEMPLATE | $YQ e "${EXTENSIONS_PATH}.repositories[] | [.url, .name] | join(\";\")")
    for REPO in $REPOS
    do
        URL=$(echo $REPO | cut -d';' -f1)
        NAME=$(echo $REPO | cut -d';' -f2)

        VERSION=$(grep -vw "{{" $TEMPLATE |
            $YQ e "${EXTENSIONS_PATH}.charts[] | select(.name == \"$NAME\") | .version")

        VALUES=$(grep -vw "{{" $TEMPLATE |
            $YQ e "${EXTENSIONS_PATH}.charts[] | select(.name == \"$NAME\") | .values")

        if [[ $URL == "" ]] || [[ $NAME == "" ]] || [[ $VERSION == "" ]]; then
            echo "Error: Failed to get URL, NAME, or VERSION from $TEMPLATE"
            exit 1
        fi

        # Check to see if a custom image is being used.
        CUSTOM_TAG=$(echo -e "$VALUES" | $YQ e '.image.tag' -)

        # Use 'helm template' to get the images used by the extension.
        for IMAGE in $($HELM template --repo $URL --version $VERSION $NAME | $YQ -N e .spec.template.spec.containers[].image);
        do
            if [[ $CUSTOM_TAG != "null" ]] || [[ -z $CUSTOM_TAG ]]; then
                IMAGE=$(echo $IMAGE | sed -e "s/:.*$/:$CUSTOM_TAG/")
            fi

            docker pull $IMAGE
            if [[ $? -ne 0 ]]; then
                echo "Error: Failed to pull $IMAGE"
                exit 1
            fi

            IMAGES_BUNDLED="$IMAGES_BUNDLED $IMAGE"
        done
    done
done

echo -e "\nSaving bundled images to $BUNDLE_TARBALL...\n"
IMAGES_BUNDLED_UNIQ=$(echo "$IMAGES_BUNDLED" | tr ' ' '\n' | sort -u)
IMAGE_IDS=

for IMAGE in $IMAGES_BUNDLED_UNIQ;
do
    docker inspect $IMAGE --format '{{ .Id }}'
    if [[ $? -ne 0 ]]; then
        echo "Error: Failed to get image ID for $IMAGE"
        exit 1
    fi
    IMAGE_IDS="$IMAGE_IDS $IMAGE"
done

docker save -o $BUNDLE_TARBALL $IMAGE_IDS

echo -e "\nCleaning up pulled images...\n"

Cleanup the images bundled by removing them from the local image cache.
for IMAGE in $IMAGES_BUNDLED_UNIQ;
do
    echo "Removing $IMAGE from local image cache..."
    docker rmi $IMAGE
    if [ $? -ne 0 ]; then
        # Note that we failed here but continue trying to remove the other
        # images.
        echo "Error: Failed to remove $IMAGE from local image cache"
    fi
done

echo "Done! Images bundled into $BUNDLE_TARBALL"
