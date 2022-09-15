// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"

	"github.com/Kristian-ZH/gardener-extension-logging/pkg/apis/config"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/eventlogger"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils/chart"
	gardeneriv "github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	seedChart = &chart.Chart{
		Name: "seed-bootstrap",
		Path: filepath.Join("charts", "seed-bootstrap"),
		SubCharts: []*chart.Chart{
			{
				Name:   "fluent-bit",
				Images: []string{"fluent-bit", "fluent-bit-plugin-installer", "alpine"},
				Objects: []*chart.Object{
					{Type: &appsv1.DaemonSet{}, Name: "fluent-bit"},
					{Type: &networkingv1.NetworkPolicy{}, Name: "allow-fluentbit"},
					{Type: &rbacv1.ClusterRole{}, Name: "fluent-bit-read"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "fluent-bit-read"},
					{Type: &corev1.ServiceAccount{}, Name: "fluent-bit"},
					{Type: &corev1.Service{}, Name: "fluent-bit"},
				},
			},
			{
				Name:   "loki",
				Images: []string{"loki", "loki-curator", "event-logger"},
				Objects: []*chart.Object{
					{Type: &appsv1.StatefulSet{}, Name: "loki"},
					{Type: &corev1.Service{}, Name: "loki"},
					{Type: &networkingv1.NetworkPolicy{}, Name: "allow-loki"},
					{Type: &networkingv1.NetworkPolicy{}, Name: " allow-to-loki"},
				},
			},
		},
	}
)

type seedActuator struct {
	logger       logr.Logger // logger
	chart        chart.Interface
	chartApplier gardenerkubernetes.ChartApplier
	imageVector  gardeneriv.ImageVector

	client            client.Client
	clientset         kubernetes.Interface
	gardenerClientset gardenerkubernetes.Interface
	serviceConfig     config.Configuration
}

func (a *seedActuator) InjectConfig(config *rest.Config) error {
	var err error

	a.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("could not create Kubernetes client: %w", err)
	}

	a.gardenerClientset, err = gardenerkubernetes.NewWithConfig(gardenerkubernetes.WithRESTConfig(config))
	if err != nil {
		return fmt.Errorf("could not create Gardener client: %w", err)
	}

	a.chartApplier = a.gardenerClientset.ChartApplier()

	return nil
}

func (a *seedActuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// NewSeedActuator returns an actuator responsible for the Seed Logging stack.
func NewSeedActuator(config config.Configuration) Actuator {
	return &seedActuator{
		logger:        log.Log.WithName("logging seed actuator"),
		chart:         seedChart,
		imageVector:   imagevector.ImageVector(),
		serviceConfig: config,
	}
}

// Reconcile the Extension resource.
func (a *seedActuator) Reconcile(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	lokiValues := map[string]interface{}{}
	lokiValues["authEnabled"] = false
	lokiSeedStorage := resource.MustParse("100Gi")
	lokiValues["storage"] = lokiSeedStorage

	if err := seed.ResizeOrDeleteLokiDataVolumeIfStorageNotTheSame(ctx, a.logger, a.client, lokiSeedStorage); err != nil {
		return err
	}

	hvpaEnabled := ex.Spec.HvpaEnabled

	if hvpaEnabled {
		maintenanceBegin := "220000-0000"
		maintenanceEnd := "230000-0000"
		if ex.Spec.Loki.Maintenance.Begin != "" && ex.Spec.Loki.Maintenance.End != "" {
			maintenanceBegin = ex.Spec.Loki.Maintenance.Begin
			maintenanceEnd = ex.Spec.Loki.Maintenance.End
		}

		lokiValues["hvpa"] = map[string]interface{}{
			"enabled": true,
			"maintenanceTimeWindow": map[string]interface{}{
				"begin": maintenanceBegin,
				"end":   maintenanceEnd,
			},
		}

		currentResources, err := kutil.GetContainerResourcesInStatefulSet(ctx, a.client, kutil.Key(ex.Namespace, "loki"))
		if err != nil {
			return err
		}
		if len(currentResources) != 0 && currentResources["loki"] != nil {
			lokiValues["resources"] = map[string]interface{}{
				// Copy requests only, effectively removing limits
				"loki": &corev1.ResourceRequirements{
					Requests: currentResources["loki"].Requests,
				},
			}
		}
	}

	lokiValues["priorityClassName"] = v1beta1constants.PriorityClassNameSeedSystem600

	additionalFilters := ex.Spec.FluentBit.AdditionalFilters
	additionalParsers := ex.Spec.FluentBit.AdditionalParsers
	if isShootEventLoggerEnabled(a.serviceConfig) {
		additionalFilters += eventlogger.Filter
	}

	values := map[string]interface{}{
		"fluent-bit": map[string]interface{}{
			"additionalFilters": additionalFilters,
			"additionalParsers": additionalParsers,
		},
		"loki": lokiValues,
	}

	if err := a.chart.Apply(ctx, a.chartApplier, ex.Namespace, a.imageVector, "", "", values); err != nil {
		return err
	}

	return nil
}

// Delete the Extension resource.
func (a *seedActuator) Delete(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	if err := a.chart.Delete(ctx, a.client, ex.Namespace); err != nil {
		return err
	}

	return nil
}

// Restore the Extension resource.
func (a *seedActuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	return a.Reconcile(ctx, log, ex, cluster)
}

// Migrate the Extension resource.
func (a *seedActuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	return a.Delete(ctx, log, ex, cluster)
}
