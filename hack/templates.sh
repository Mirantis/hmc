#!/bin/sh
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


set -eu

# Directory containing HMC templates
TEMPLATES_DIR=${TEMPLATES_DIR:-templates}
# Output directory for the generated Template manifests
TEMPLATES_OUTPUT_DIR=${TEMPLATES_OUTPUT_DIR:-templates/hmc-templates/files/templates}
# The name of the HMC templates helm chart
HMC_TEMPLATES_CHART_NAME='hmc-templates'

mkdir -p $TEMPLATES_OUTPUT_DIR
rm $TEMPLATES_OUTPUT_DIR/*.yaml

for chart in $TEMPLATES_DIR/*; do
    if [ -d "$chart" ]; then
        name=$(grep '^name:' $chart/Chart.yaml | awk '{print $2}')
        if [ "$name" = "$HMC_TEMPLATES_CHART_NAME" ]; then continue; fi
        version=$(grep '^version:' $chart/Chart.yaml | awk '{print $2}')

        cat <<EOF > $TEMPLATES_OUTPUT_DIR/$name.yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Template
metadata:
  name: $name
spec:
  helm:
    chartName: $name
    chartVersion: $version
EOF

        echo "Generated $TEMPLATES_OUTPUT_DIR/$name.yaml"
    fi
done
