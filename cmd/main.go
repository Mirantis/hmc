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

package main

import (
	"crypto/tls"
	"flag"
	"os"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sveltosv1beta1 "github.com/projectsveltos/addon-controller/api/v1beta1"
	velerov1api "github.com/zerospiel/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/zerospiel/velero/pkg/apis/velero/v2alpha1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capo "sigs.k8s.io/cluster-api-provider-openstack/api/v1beta1"
	capv "sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	hmcmirantiscomv1alpha1 "github.com/Mirantis/hmc/api/v1alpha1"
	"github.com/Mirantis/hmc/internal/controller"
	"github.com/Mirantis/hmc/internal/helm"
	"github.com/Mirantis/hmc/internal/telemetry"
	"github.com/Mirantis/hmc/internal/utils"
	hmcwebhook "github.com/Mirantis/hmc/internal/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// velero deps
	utilruntime.Must(velerov1api.AddToScheme(scheme))
	utilruntime.Must(velerov2alpha1api.AddToScheme(scheme))
	utilruntime.Must(apiextv1.AddToScheme(scheme))
	utilruntime.Must(apiextv1beta1.AddToScheme(scheme))
	// WARN: if snapshot is to be used, then the following resources should also be added to the scheme
	// snapshotv1api.AddToScheme(scheme) // snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	// velero deps

	utilruntime.Must(hmcmirantiscomv1alpha1.AddToScheme(scheme))
	utilruntime.Must(sourcev1.AddToScheme(scheme))
	utilruntime.Must(hcv2.AddToScheme(scheme))
	utilruntime.Must(sveltosv1beta1.AddToScheme(scheme))
	utilruntime.Must(capz.AddToScheme(scheme))
	utilruntime.Must(capv.AddToScheme(scheme))
	utilruntime.Must(capo.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr               string
		probeAddr                 string
		secureMetrics             bool
		enableHTTP2               bool
		defaultRegistryURL        string
		insecureRegistry          bool
		registryCredentialsSecret string
		createManagement          bool
		createAccessManagement    bool
		createRelease             bool
		createTemplates           bool
		hmcTemplatesChartName     string
		enableTelemetry           bool
		enableWebhook             bool
		webhookPort               int
		webhookCertDir            string
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&defaultRegistryURL, "default-registry-url", "oci://ghcr.io/mirantis/hmc/charts",
		"The default registry to download Helm charts from, prefix with oci:// for OCI registries.")
	flag.StringVar(&registryCredentialsSecret, "registry-creds-secret", "",
		"Secret containing authentication credentials for the registry.")
	flag.BoolVar(&insecureRegistry, "insecure-registry", false, "Allow connecting to an HTTP registry.")
	flag.BoolVar(&createManagement, "create-management", true, "Create a Management object with default configuration upon initial installation.")
	flag.BoolVar(&createAccessManagement, "create-access-management", true,
		"Create an AccessManagement object upon initial installation.")
	flag.BoolVar(&createRelease, "create-release", true, "Create an HMC Release upon initial installation.")
	flag.BoolVar(&createTemplates, "create-templates", true, "Create HMC Templates based on Release objects.")
	flag.StringVar(&hmcTemplatesChartName, "hmc-templates-chart-name", "hmc-templates",
		"The name of the helm chart with HMC Templates.")
	flag.BoolVar(&enableTelemetry, "enable-telemetry", true, "Collect and send telemetry data.")
	flag.BoolVar(&enableWebhook, "enable-webhook", true, "Enable admission webhook.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "Admission webhook port.")
	flag.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	determinedRepositoryType, err := utils.DetermineDefaultRepositoryType(defaultRegistryURL)
	if err != nil {
		setupLog.Error(err, "failed to determine default repository type")
		os.Exit(1)
	}

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	managerOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         true,
		LeaderElectionID:       "31c555b4.hmc.mirantis.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	}

	if enableWebhook {
		managerOpts.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			TLSOpts: tlsOpts,
			CertDir: webhookCertDir,
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	dc, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "failed to create dynamic client")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	if err = hmcmirantiscomv1alpha1.SetupIndexers(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to setup indexers")
		os.Exit(1)
	}

	currentNamespace := utils.CurrentNamespace()

	templateReconciler := controller.TemplateReconciler{
		Client:          mgr.GetClient(),
		SystemNamespace: currentNamespace,
		DefaultRegistryConfig: helm.DefaultRegistryConfig{
			URL:               defaultRegistryURL,
			RepoType:          determinedRepositoryType,
			CredentialsSecret: registryCredentialsSecret,
			Insecure:          insecureRegistry,
		},
	}

	if err = (&controller.ClusterTemplateReconciler{
		TemplateReconciler: templateReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterTemplate")
		os.Exit(1)
	}
	if err = (&controller.ServiceTemplateReconciler{
		TemplateReconciler: templateReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceTemplate")
		os.Exit(1)
	}
	if err = (&controller.ProviderTemplateReconciler{
		TemplateReconciler: templateReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ProviderTemplate")
		os.Exit(1)
	}
	if err = (&controller.ClusterDeploymentReconciler{
		Client:          mgr.GetClient(),
		Config:          mgr.GetConfig(),
		DynamicClient:   dc,
		SystemNamespace: currentNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterDeployment")
		os.Exit(1)
	}
	if err = (&controller.ManagementReconciler{
		Client:                 mgr.GetClient(),
		Scheme:                 mgr.GetScheme(),
		Config:                 mgr.GetConfig(),
		DynamicClient:          dc,
		SystemNamespace:        currentNamespace,
		CreateAccessManagement: createAccessManagement,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Management")
		os.Exit(1)
	}
	if err = (&controller.AccessManagementReconciler{
		Client:          mgr.GetClient(),
		Config:          mgr.GetConfig(),
		SystemNamespace: currentNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AccessManagement")
		os.Exit(1)
	}

	templateChainReconciler := controller.TemplateChainReconciler{
		Client:          mgr.GetClient(),
		SystemNamespace: currentNamespace,
	}
	if err = (&controller.ClusterTemplateChainReconciler{
		TemplateChainReconciler: templateChainReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterTemplateChain")
		os.Exit(1)
	}
	if err = (&controller.ServiceTemplateChainReconciler{
		TemplateChainReconciler: templateChainReconciler,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ServiceTemplateChain")
		os.Exit(1)
	}

	if err = (&controller.ReleaseReconciler{
		Client:                mgr.GetClient(),
		Config:                mgr.GetConfig(),
		CreateManagement:      createManagement,
		CreateRelease:         createRelease,
		CreateTemplates:       createTemplates,
		HMCTemplatesChartName: hmcTemplatesChartName,
		SystemNamespace:       currentNamespace,
		DefaultRegistryConfig: helm.DefaultRegistryConfig{
			URL:               defaultRegistryURL,
			RepoType:          determinedRepositoryType,
			CredentialsSecret: registryCredentialsSecret,
			Insecure:          insecureRegistry,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Release")
		os.Exit(1)
	}

	if enableTelemetry {
		if err = mgr.Add(&telemetry.Tracker{
			Client:          mgr.GetClient(),
			SystemNamespace: currentNamespace,
		}); err != nil {
			setupLog.Error(err, "unable to create telemetry tracker")
			os.Exit(1)
		}
	}

	if err = (&controller.CredentialReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Credential")
		os.Exit(1)
	}

	if err = (&controller.MultiClusterServiceReconciler{
		Client:          mgr.GetClient(),
		SystemNamespace: currentNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MultiClusterService")
		os.Exit(1)
	}
	// TODO (zerospiel): disabled until the #605
	// if err = (&controller.ManagementBackupReconciler{
	// 	Client: mgr.GetClient(),
	// }).SetupWithManager(mgr); err != nil {
	// 	setupLog.Error(err, "unable to create controller", "controller", "ManagementBackup")
	// 	os.Exit(1)
	// }
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if enableWebhook {
		if err := setupWebhooks(mgr, currentNamespace); err != nil {
			setupLog.Error(err, "failed to setup webhooks")
			os.Exit(1)
		}
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupWebhooks(mgr ctrl.Manager, currentNamespace string) error {
	if err := (&hmcwebhook.ClusterDeploymentValidator{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterDeployment")
		return err
	}
	if err := (&hmcwebhook.MultiClusterServiceValidator{SystemNamespace: currentNamespace}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "MultiClusterService")
		return err
	}
	if err := (&hmcwebhook.ManagementValidator{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Management")
		return err
	}
	if err := (&hmcwebhook.AccessManagementValidator{SystemNamespace: currentNamespace}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "AccessManagement")
		return err
	}
	if err := (&hmcwebhook.ClusterTemplateChainValidator{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterTemplateChain")
		return err
	}
	if err := (&hmcwebhook.ServiceTemplateChainValidator{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ServiceTemplateChain")
		return err
	}

	templateValidator := hmcwebhook.TemplateValidator{
		SystemNamespace: currentNamespace,
	}
	if err := (&hmcwebhook.ClusterTemplateValidator{TemplateValidator: templateValidator}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ClusterTemplate")
		return err
	}
	if err := (&hmcwebhook.ServiceTemplateValidator{TemplateValidator: templateValidator}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ServiceTemplate")
		return err
	}
	if err := (&hmcwebhook.ProviderTemplateValidator{TemplateValidator: templateValidator}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "ProviderTemplate")
		return err
	}
	if err := (&hmcwebhook.ReleaseValidator{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Release")
		return err
	}
	return nil
}
