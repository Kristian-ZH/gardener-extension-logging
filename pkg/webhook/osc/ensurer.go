// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package osc

import (
	"context"

	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	oidcWebhookConfigPrefix               = "--authentication-token-webhook-config-file="
	oidcWebhookCacheTTLPrefix             = "--authentication-token-webhook-cache-ttl="
	oidcAuthenticatorKubeConfigVolumeName = "oidc-webhook-authenticator-kubeconfig"
	tokenValidatorSecretVolumeName        = "token-validator-secret"
)

type ensurer struct {
	genericmutator.NoopEnsurer
	client client.Client
	logger logr.Logger
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client

	return nil
}

// NewEnsurer creates a new logging mutator.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("logging-ensurer"),
	}
}

// EnsureAdditionalUnits ensures that additional required system units are added.
func (e *ensurer) EnsureAdditionalUnits(ctx context.Context, gctx gcontext.GardenContext, new, old *[]extensionsv1alpha1.Unit) error {
	promtailCM, err := getPromtailCM(ctx, gctx, e.client)
	if err != nil {
		return err
	}

	var unitsToAdd []extensionsv1alpha1.Unit

	if promtailCM.BinaryData != nil {
		if err := yaml.Unmarshal(promtailCM.BinaryData["promtailUnits"], &unitsToAdd); err != nil {
			return err
		}
	}

	for _, unit := range unitsToAdd {
		appendUniqueUnit(new, unit)
	}

	return nil
}

// EnsureAdditionalFiles ensures that additional required system files are added.
func (e *ensurer) EnsureAdditionalFiles(ctx context.Context, gctx gcontext.GardenContext, new, _ *[]extensionsv1alpha1.File) error {
	promtailCM, err := getPromtailCM(ctx, gctx, e.client)
	if err != nil {
		return err
	}

	var filesToAdd []extensionsv1alpha1.File

	if promtailCM.BinaryData != nil {
		if err := yaml.Unmarshal(promtailCM.BinaryData["promtailFiles"], &filesToAdd); err != nil {
			return err
		}
	}

	for _, file := range filesToAdd {
		appendUniqueFile(new, file)
	}

	return nil
}

func getPromtailCM(ctx context.Context, gctx gcontext.GardenContext, seedClient client.Client) (*corev1.ConfigMap, error) {
	cluster, err := gctx.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	promtailCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "promtail-config", Namespace: cluster.ObjectMeta.Name}}
	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(promtailCM), promtailCM); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	return promtailCM, nil
}

// appendUniqueUnit appends a unit only if it does not exist, otherwise overwrite content of previous units
func appendUniqueUnit(units *[]extensionsv1alpha1.Unit, unit extensionsv1alpha1.Unit) {
	resUnits := make([]extensionsv1alpha1.Unit, 0, len(*units))

	for _, u := range *units {
		if u.Name != unit.Name {
			resUnits = append(resUnits, u)
		}
	}

	*units = append(resUnits, unit)
}

// appendUniqueFile appends a unit file only if it does not exist, otherwise overwrite content of previous files
func appendUniqueFile(files *[]extensionsv1alpha1.File, file extensionsv1alpha1.File) {
	resFiles := make([]extensionsv1alpha1.File, 0, len(*files))

	for _, f := range *files {
		if f.Path != file.Path {
			resFiles = append(resFiles, f)
		}
	}

	*files = append(resFiles, file)
}
