// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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
)

// DefaultAddOptions contains configuration for the mwe controller
var DefaultAddOptions = AddOptions{}

// AddOptions are options to apply when adding the mwe controller to the manager.
type AddOptions struct {
	// SeedActuator is an seed actuator.
	SeedActuator Actuator
	// ShootActuator is an shoot actuator.
	ShootActuator Actuator
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Types []string
	// ControllerOptions contains options for the controller.
	ControllerOptions controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
}

// AddToManager adds a mwe Lifecycle controller to the given Controller Manager.
func AddToManager(mgr manager.Manager) error {
	return Add(mgr, AddOptions{
		SeedActuator:      NewSeedActuator(),
		ShootActuator:     NewShootActuator(),
		ControllerOptions: DefaultAddOptions.ControllerOptions,
		Predicates: extensionspredicate.DefaultControllerPredicates(DefaultAddOptions.IgnoreOperationAnnotation,
			predicate.Or(
				extensionspredicate.IsInGardenNamespacePredicate,
				extensionspredicate.ShootNotFailedPredicate())),
		Types: []string{"seed", "shoot"},
	})
}

func Add(mgr manager.Manager, args AddOptions) error {
	args.ControllerOptions.Reconciler = NewReconciler(args.SeedActuator, args.ShootActuator)
	args.ControllerOptions.RecoverPanic = true

	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Types...)
	// if args.IgnoreOperationAnnotation {
	// 	if err := ctrl.Watch(
	// 		&source.Kind{Type: &extensionsv1alpha1.Cluster{}},
	// 		mapper.EnqueueRequestsFrom(ClusterToDNSRecordMapper(predicates), mapper.UpdateWithNew, ctrl.GetLogger()),
	// 	); err != nil {
	// 		return err
	// 	}
	// }

	return ctrl.Watch(&source.Kind{Type: &extensionsv1alpha1.Logging{}}, &handler.EnqueueRequestForObject{}, predicates...)
}
