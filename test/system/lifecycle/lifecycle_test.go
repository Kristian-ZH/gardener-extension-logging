// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

/**
	Overview
		- Tests the lifecycle controller for the logging-service extension.
	Prerequisites
		- A Shoot exists and the logging extension is available for the seed cluster.
**/

package lifecycle_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/gardener/gardener/test/framework"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	framework.RegisterShootFrameworkFlags()
}

const (
	timeout         = 30 * time.Minute
	gardenNamespace = "garden"
	seedType        = "seed"
	shootType       = "shoot"
)

var _ = Describe("Shoot logging service testing", func() {
	f := framework.NewShootFramework(nil)

	f.Serial().Beta().CIt("Should perform the common case scenario without any errors", func(ctx context.Context) {
		// Verify seed logging
		verifyLoggingDeployment(ctx, f, seedType, gardenNamespace)
		// Verify shoot logging
		verifyLoggingDeployment(ctx, f, shootType, f.ShootSeedNamespace())
	}, timeout)
})

func verifyLoggingDeployment(ctx context.Context, f *framework.ShootFramework, resType, namespace string) {
	loggingResource := generateLoggingResource(namespace, resType)
	err := f.SeedClient.Client().Create(ctx, loggingResource)
	Expect(err).ToNot(HaveOccurred())

	loggingDeployment := generateLoggingDeployment(namespace, resType)

	err = f.WaitUntilDeploymentIsReady(ctx, loggingDeployment.ObjectMeta.Name, loggingDeployment.ObjectMeta.Namespace, f.SeedClient)
	Expect(err).ToNot(HaveOccurred())
	err = f.SeedClient.Client().Get(ctx, client.ObjectKeyFromObject(loggingDeployment), loggingDeployment)
	Expect(err).ToNot(HaveOccurred())
	one := int32(1)
	Expect(*loggingDeployment.Spec.Replicas).To(BeNumerically(">=", one))
	Expect(loggingDeployment.Status.ReadyReplicas).To(BeNumerically(">=", one))

	// Verify logging deletion process
	err = f.SeedClient.Client().Delete(ctx, loggingDeployment)
	Expect(err).ToNot(HaveOccurred())

	err = f.SeedClient.Client().Get(ctx, client.ObjectKeyFromObject(loggingDeployment), loggingDeployment)
	Expect(err).To(HaveOccurred())
	Expect(err).To(BeNotFoundError())
}

func generateLoggingResource(namespace, resType string) *extensionsv1alpha1.Logging {
	loggingResource := &extensionsv1alpha1.Logging{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Logging",
			APIVersion: "extensions.gardener.cloud/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-" + resType + "-extension",
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.LoggingSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: resType,
			},
		},
	}

	return loggingResource
}

func generateLoggingDeployment(namespace, resType string) *appsv1.Deployment {
	loggingDeployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-world-" + resType,
			Namespace: namespace,
		},
	}

	return loggingDeployment
}
