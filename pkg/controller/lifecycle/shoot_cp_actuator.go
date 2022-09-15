// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"

	"github.com/Kristian-ZH/gardener-extension-logging/pkg/apis/config"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/eventlogger"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/kuberbacproxy"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/imagevector"
	"github.com/Kristian-ZH/gardener-extension-logging/pkg/secrets"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	gardenerkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/images"
	gardeneriv "github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	shootChart = &chart.Chart{
		Name: "shoot-bootstrap",
		Path: filepath.Join("charts", "shoot-bootstrap"),
		SubCharts: []*chart.Chart{
			{
				Name:   "loki",
				Images: []string{"loki", "loki-curator", "kube-rbac-proxy", "telegraf", "event-logger"},
				Objects: []*chart.Object{
					{Type: &appsv1.StatefulSet{}, Name: "loki"},
					{Type: &corev1.Service{}, Name: "loki"},
					{Type: &networkingv1.NetworkPolicy{}, Name: "allow-loki"},
					{Type: &networkingv1.NetworkPolicy{}, Name: " allow-to-loki"},
					{Type: &networkingv1.NetworkPolicy{}, Name: "allow-from-prometheus-to-loki-telegraf"},
					{Type: &networkingv1.Ingress{}, Name: "loki"},
				},
			},
		},
	}
)

type shootActuator struct {
	logger       logr.Logger // logger
	chart        chart.Interface
	chartApplier gardenerkubernetes.ChartApplier
	imageVector  gardeneriv.ImageVector

	client            client.Client
	clientset         kubernetes.Interface
	gardenerClientset gardenerkubernetes.Interface
	serviceConfig     config.Configuration
}

func (a *shootActuator) InjectConfig(config *rest.Config) error {
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

func (a *shootActuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// NewShootActuator returns an actuator responsible for the Shoot Logging stack.
func NewShootActuator(config config.Configuration) Actuator {
	return &shootActuator{
		logger:        log.Log.WithName("logging shoot actuator"),
		chart:         shootChart,
		imageVector:   imagevector.ImageVector(),
		serviceConfig: config,
	}
}

// Reconcile the Extension resource.
func (a *shootActuator) Reconcile(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	lokiValues := map[string]interface{}{}
	hvpaEnabled := ex.Spec.HvpaEnabled

	genericTokenKubeconfigSecret := extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster)

	imageEventLogger, err := a.imageVector.FindImage(images.ImageNameEventLogger)
	if err != nil {
		return err
	}
	eventLogger, err := eventlogger.New(a.client, ex.Namespace, genericTokenKubeconfigSecret, eventlogger.Values{Image: imageEventLogger.String(), Replicas: 1})
	if err != nil {
		return err
	}

	shootRBACProxy, err := kuberbacproxy.New(a.client, ex.Namespace)
	if err != nil {
		return err
	}

	if isShootEventLoggerEnabled(a.serviceConfig) {
		if err := eventLogger.Deploy(ctx); err != nil {
			return err
		}
	} else {
		if err := eventLogger.Destroy(ctx); err != nil {
			return err
		}
	}

	if isShootNodeLoggingEnabled(a.serviceConfig, cluster.Shoot.Spec.Purpose) {
		if err := shootRBACProxy.Deploy(ctx); err != nil {
			return err
		}

		ingressClass, err := ComputeNginxIngressClass(cluster.Seed, cluster.Seed.Status.KubernetesVersion)
		if err != nil {
			return err
		}

		configs := secrets.ConfigsFor(ex.Namespace, cluster)

		secretsManager, err := extensionssecretsmanager.SecretsManagerForCluster(ctx, logger.WithName("secretsmanager"), clock.RealClock{}, a.client, cluster, secrets.ManagerIdentity, configs)
		if err != nil {
			return err
		}

		_, err = extensionssecretsmanager.GenerateAllSecrets(ctx, secretsManager, configs)
		if err != nil {
			return err
		}

		caBundleSecret, found := secretsManager.Get(secrets.CAName)
		if !found {
			return fmt.Errorf("secret %q not found", secrets.CAName)
		}

		lokiValues["rbacSidecarEnabled"] = true
		lokiValues["ingress"] = map[string]interface{}{
			"class": ingressClass,
			"hosts": []map[string]interface{}{
				{
					"hostName":    secrets.ComputeIngressHost(cluster.Shoot, cluster.Seed, "l"),
					"secretName":  caBundleSecret.Name,
					"serviceName": "loki",
					"servicePort": 8080,
					"backendPath": "/loki/api/v1/push",
				},
			},
		}
		lokiValues["genericTokenKubeconfigSecretName"] = genericTokenKubeconfigSecret

		if err := secretsManager.Cleanup(ctx); err != nil {
			return err
		}
	} else {
		if err := shootRBACProxy.Destroy(ctx); err != nil {
			return err
		}

		return kutil.DeleteObjects(ctx, a.client,
			&extensionsv1beta1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: ex.Namespace}},
			&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: ex.Namespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: ex.Namespace}},
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: ex.Namespace}},
		)
	}

	lokiValues["priorityClassName"] = v1beta1constants.PriorityClassNameShootControlPlane100

	hvpaValues := make(map[string]interface{})
	hvpaValues["enabled"] = hvpaEnabled
	lokiValues["hvpa"] = hvpaValues

	if hvpaEnabled {
		currentResources, err := kutil.GetContainerResourcesInStatefulSet(ctx, a.client, kutil.Key(ex.Namespace, "loki"))
		if err != nil {
			return err
		}
		if len(currentResources) != 0 && currentResources["loki"] != nil {
			lokiValues["resources"] = map[string]interface{}{
				// Copy requests only, effectively removing limits
				"loki": &corev1.ResourceRequirements{Requests: currentResources["loki"].Requests},
			}
		}
	}

	values := map[string]interface{}{
		"loki": lokiValues,
	}
	if err := a.chart.Apply(ctx, a.chartApplier, ex.Namespace, a.imageVector, "", "", values); err != nil {
		return err
	}

	return nil
}

// Delete the Extension resource.
func (a *shootActuator) Delete(ctx context.Context, logger logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	imageEventLogger, err := a.imageVector.FindImage(images.ImageNameEventLogger)
	if err != nil {
		return err
	}

	eventLogger, err := eventlogger.New(a.client, ex.Namespace, "", eventlogger.Values{Image: imageEventLogger.String(), Replicas: 1})
	if err != nil {
		return err
	}

	if err := eventLogger.Destroy(ctx); err != nil {
		return err
	}

	shootRBACProxy, err := kuberbacproxy.New(a.client, ex.Namespace)
	if err != nil {
		return err
	}

	if err := shootRBACProxy.Destroy(ctx); err != nil {
		return err
	}

	secretsManager, err := extensionssecretsmanager.SecretsManagerForCluster(ctx, logger.WithName("secretsmanager"), clock.RealClock{}, a.client, cluster, secrets.ManagerIdentity, nil)
	if err != nil {
		return err
	}

	if err := secretsManager.Cleanup(ctx); err != nil {
		return err
	}

	if err := a.chart.Delete(ctx, a.client, ex.Namespace); err != nil {
		return err
	}

	return nil
}

// Restore the Extension resource.
func (a *shootActuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	return a.Reconcile(ctx, log, ex, cluster)
}

// Migrate the Extension resource.
func (a *shootActuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Logging, cluster *extensions.Cluster) error {
	return a.Delete(ctx, log, ex, cluster)
}
