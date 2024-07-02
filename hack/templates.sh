#!/bin/sh

set -eu

# Directory containing HMC templates
TEMPLATES_DIR=${TEMPLATES_DIR:-templates}
# Output directory for the generated Template manifests
TEMPLATES_OUTPUT_DIR=${TEMPLATES_OUTPUT_DIR:-templates/hmc-templates/files/templates}

mkdir -p $TEMPLATES_OUTPUT_DIR
rm $TEMPLATES_OUTPUT_DIR/*.yaml

for chart in $TEMPLATES_DIR/*; do
    if [ -d "$chart" ]; then
        name=$(grep '^name:' $chart/Chart.yaml | awk '{print $2}')
        appVersion=$(grep '^appVersion:' $chart/Chart.yaml | awk '{print $2}')

        cat <<EOF > $TEMPLATES_OUTPUT_DIR/$name.yaml
apiVersion: hmc.mirantis.com/v1alpha1
kind: Template
metadata:
  name: $name
spec:
  helm:
    chartName: $name
    chartVersion: $appVersion
EOF

        echo "Generated $TEMPLATES_OUTPUT_DIR/$name.yaml"
    fi
done
