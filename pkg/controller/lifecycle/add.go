// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"strings"

	controllerconfig "github.com/Kristian-ZH/gardener-extension-logging/pkg/controller/config"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// FinalizerName is the dnsrecord controller finalizer.
	FinalizerName = "extensions.gardener.cloud/logging"
	// ControllerName is the name of the controller
	ControllerName = "logging"
	// Name is the name of the lifecycle controller.
	Name = "logging_lifecycle_controller"
)

// DefaultAddOptions contains configuration for the mwe controller
var DefaultAddOptions = AddOptions{}

// IsInGardenorShootNamespacePredicate is a predicate which returns true when the provided object is in the 'garden' or in the shoot namespaces.
var IsInGardenorShootNamespacePredicate = predicate.NewPredicateFuncs(func(obj client.Object) bool {
	return obj != nil && (obj.GetNamespace() == "garden" || strings.HasPrefix(obj.GetNamespace(), "shoot--"))
})

// AddOptions are options to apply when adding the mwe controller to the manager.
type AddOptions struct {
	// SeedActuator is an seed actuator.
	SeedActuator Actuator
	// ShootActuator is an shoot actuator.
	ShootActuator Actuator
	// Name is the name of the controller.
	Name string
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Types []string
	// ControllerOptions contains options for the controller.
	ControllerOptions controller.Options
	// ServiceConfig contains configuration for the shoot OIDC service.
	ServiceConfig controllerconfig.Config
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManager adds a logging Lifecycle controller to the given Controller Manager.
func AddToManager(mgr manager.Manager) error {
	return Add(mgr, AddOptions{
		SeedActuator:      NewSeedActuator(DefaultAddOptions.ServiceConfig.Configuration),
		ShootActuator:     NewShootActuator(DefaultAddOptions.ServiceConfig.Configuration),
		Name:              Name,
		ControllerOptions: DefaultAddOptions.ControllerOptions,
		Predicates:        extensionspredicate.DefaultControllerPredicates(DefaultAddOptions.IgnoreOperationAnnotation, IsInGardenorShootNamespacePredicate),
		Types:             []string{"seed", "shoot"},
	})
}

// Add creates the Reconciler and connects it to the resources
func Add(mgr manager.Manager, args AddOptions) error {
	args.ControllerOptions.Reconciler = NewReconciler(args.SeedActuator, args.ShootActuator)
	args.ControllerOptions.RecoverPanic = true

	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Types...)
	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(
			&source.Kind{Type: &extensionsv1alpha1.Cluster{}},
			mapper.EnqueueRequestsFrom(extension.ClusterToExtensionMapper(predicates...), mapper.UpdateWithNew, mgr.GetLogger().WithName(args.Name)),
		); err != nil {
			return err
		}
	}

	return ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Logging{}}, &handler.EnqueueRequestForObject{}, predicates...)
}
