## Install applications into Target Cluster

To install applications into the target cluster created using Cluster API (CAPI) upon creation, a Flux `HelmRelease` object is to be made such that its `.spec.KubeConfig` references the kubeconfig of the target cluster.

**Reference:** https://fluxcd.io/flux/components/helm/helmreleases/#remote-clusters--cluster-api

This chart/template already defines the following applications under `templates/beachheadservices` which can be be installed into the target cluster by setting `.Values.installBeachHeadServices=true`:
1. cert-manager
2. nginx-ingress

**Important:** The Flux objects added to `templates/beachheadservices` to install custom applications must have the `hmc.mirantis.com/managed: "true"` label to be reconciled by HMC.
