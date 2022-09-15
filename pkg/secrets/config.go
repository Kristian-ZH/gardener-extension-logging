// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"fmt"
	"strings"
	"time"

	"github.com/Kristian-ZH/gardener-extension-logging/pkg/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/extensions"

	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

const (
	// ManagerIdentity is the identity used for the secrets manager.
	ManagerIdentity = "extension-" + constants.ExtensionType
	// CAName is the name of the CA secret.
	CAName = "ca-extension-" + constants.ExtensionType
)

var (
	ingressTLSCertificateValidity = 730 * 24 * time.Hour
)

// ConfigsFor returns configurations for the secrets manager for the given namespace.
func ConfigsFor(namespace string, cluster *extensions.Cluster) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       CAName,
				CommonName: CAName,
				CertType:   secretutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        "loki-tls",
				CommonName:                  ComputeIngressHost(cluster.Shoot, cluster.Seed, "l"),
				Organization:                []string{"gardener.cloud:monitoring:ingress"},
				DNSNames:                    ComputeLokiHosts(cluster.Shoot, cluster.Seed, "l"),
				CertType:                    secretutils.ServerCert,
				Validity:                    &ingressTLSCertificateValidity,
				SkipPublishingCACertificate: true,
			},
			// use current CA for signing server cert to prevent mismatches when dropping the old CA from the webhook
			// config in phase Completing
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(CAName, secretsmanager.UseCurrentCA)},
		},
	}
}

// ComputeIngressHost computes the host for a given prefix.
func ComputeIngressHost(shoot *v1beta1.Shoot, seed *v1beta1.Seed, prefix string) string {
	shortID := strings.Replace(shoot.Status.TechnicalID, shootpkg.TechnicalIDPrefix, "", 1)
	return fmt.Sprintf("%s-%s.%s", prefix, shortID, IngressDomain(seed))
}

// ComputeLokiHosts computes the host for loki.
func ComputeLokiHosts(shoot *v1beta1.Shoot, seed *v1beta1.Seed, prefix string) []string {
	return []string{
		ComputeIngressHost(shoot, seed, prefix),
	}
}

// IngressDomain returns the ingress domain for the seed.
func IngressDomain(seed *v1beta1.Seed) string {
	if seed.Spec.DNS.IngressDomain != nil {
		return *seed.Spec.DNS.IngressDomain
	} else if seed.Spec.Ingress != nil {
		return seed.Spec.Ingress.Domain
	}
	return ""
}
