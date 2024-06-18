package helm

import (
	"context"
	"time"

	hcv2 "github.com/fluxcd/helm-controller/api/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileHelmRelease(
	ctx context.Context,
	cl client.Client,
	name string,
	namespace string,
	values *apiextensionsv1.JSON,
	ownerReference metav1.OwnerReference,
	chartRef *hcv2.CrossNamespaceSourceReference,
	reconcileInterval time.Duration,
) (*hcv2.HelmRelease, error) {
	helmRelease := &hcv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, cl, helmRelease, func() error {
		helmRelease.OwnerReferences = []metav1.OwnerReference{ownerReference}
		helmRelease.Spec = hcv2.HelmReleaseSpec{
			ChartRef:    chartRef,
			Interval:    metav1.Duration{Duration: reconcileInterval},
			ReleaseName: name,
			Values:      values,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return helmRelease, nil
}
