// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

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
	"github.com/gardener/gardener/pkg/utils/timewindow"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	rewriteTagRegex = regexp.MustCompile(`\$tag\s+(.+?)\s+user-exposed\.\$TAG\s+true`)

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
		shootInfo := &corev1.ConfigMap{}
		maintenanceBegin := "220000-0000"
		maintenanceEnd := "230000-0000"
		if err := a.client.Get(ctx, kutil.Key(metav1.NamespaceSystem, v1beta1constants.ConfigMapNameShootInfo), shootInfo); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		} else {
			shootMaintenanceBegin, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceBegin"])
			if err != nil {
				return err
			}
			maintenanceBegin = shootMaintenanceBegin.Add(1, 0, 0).Formatted()

			shootMaintenanceEnd, err := timewindow.ParseMaintenanceTime(shootInfo.Data["maintenanceEnd"])
			if err != nil {
				return err
			}
			maintenanceEnd = shootMaintenanceEnd.Add(1, 0, 0).Formatted()
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

	additionalFilters := strings.Builder{}
	additionalParsers := strings.Builder{}

	if isShootEventLoggerEnabled(a.serviceConfig) {
		additionalFilters.WriteString(eventlogger.Filter)
	}

	// Read extension provider specific logging configuration
	existingConfigMaps := &corev1.ConfigMapList{}
	if err := a.client.List(ctx, existingConfigMaps,
		client.InNamespace(v1beta1constants.GardenNamespace),
		client.MatchingLabels{v1beta1constants.LabelExtensionConfiguration: v1beta1constants.LabelLogging}); err != nil {
		return err
	}

	// Need stable order before passing the dashboards to Grafana config to avoid unnecessary changes
	kutil.ByName().Sort(existingConfigMaps)
	modifyFilter := `
    Name          modify
    Match         kubernetes.*
    Condition     Key_value_matches tag __PLACE_HOLDER__
    Add           __gardener_multitenant_id__ operator;user
`
	// Read all filters and parsers coming from the extension provider configurations
	for _, cm := range existingConfigMaps.Items {
		// Remove the extensions rewrite_tag filters.
		// TODO (vlvasilev): When all custom rewrite_tag filters are removed from the extensions this code snipped must be removed
		flbFilters := cm.Data[v1beta1constants.FluentBitConfigMapKubernetesFilter]
		tokens := strings.Split(flbFilters, "[FILTER]")
		var sb strings.Builder
		for _, token := range tokens {
			if strings.Contains(token, "rewrite_tag") {
				result := rewriteTagRegex.FindAllStringSubmatch(token, 1)
				if len(result) < 1 || len(result[0]) < 2 {
					continue
				}
				token = strings.Replace(modifyFilter, "__PLACE_HOLDER__", result[0][1], 1)
			}
			// In case we are processing the first token
			if strings.TrimSpace(token) != "" {
				sb.WriteString("[FILTER]")
			}
			sb.WriteString(token)
		}
		additionalFilters.WriteString(fmt.Sprintln(strings.TrimRight(sb.String(), " ")))
		additionalParsers.WriteString(fmt.Sprintln(cm.Data[v1beta1constants.FluentBitConfigMapParser]))
	}

	values := map[string]interface{}{
		"fluent-bit": map[string]interface{}{
			"additionalFilters": additionalFilters.String(),
			"additionalParsers": additionalParsers.String(),
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
