/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ReconcileRequestedPredicate struct {
	predicate.Funcs
}

// Update implements the default UpdateEvent filter for validating flux_utils.ReconcileRequestAnnotation changes.
func (ReconcileRequestedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if val, ok := e.ObjectNew.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; ok {
		if valOld, okOld := e.ObjectOld.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; okOld {
			return val != valOld
		}
		return true
	}
	return false
}

type DeployRequestedPredicate struct {
	predicate.Funcs
}

func (DeployRequestedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil || e.ObjectNew == nil {
		return false
	}

	if val, ok := e.ObjectNew.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; ok {
		if valOld, okOld := e.ObjectOld.GetAnnotations()[kluctlv1.KluctlRequestReconcileAnnotation]; okOld {
			return val != valOld
		}
		return true
	}
	if val, ok := e.ObjectNew.GetAnnotations()[kluctlv1.KluctlRequestDeployAnnotation]; ok {
		if valOld, okOld := e.ObjectOld.GetAnnotations()[kluctlv1.KluctlRequestDeployAnnotation]; okOld {
			return val != valOld
		}
		return true
	}
	return false
}
