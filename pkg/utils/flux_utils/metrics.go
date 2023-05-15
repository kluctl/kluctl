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

package flux_utils

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/conditions"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/metrics"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
)

// Metrics is a helper struct that adds the capability for recording GitOps Toolkit standard metrics to a reconciler.
//
// Use it by embedding it in your reconciler struct:
//
//		type MyTypeReconciler {
//	 	client.Client
//	     // ... etc.
//	     controller.Metrics
//		}
//
// Following the GitOps Toolkit conventions, API types used in GOTK SHOULD implement conditions.Getter to work with
// status condition types, and this convention MUST be followed to be able to record metrics using this helper.
//
// Use MustMakeMetrics to create a working Metrics value; you can supply the same value to all reconcilers.
//
// Once initialised, metrics can be recorded by calling one of the available `Record*` methods.
type Metrics struct {
	Scheme          *runtime.Scheme
	MetricsRecorder *metrics.Recorder
}

// RecordDuration records the duration of a reconcile attempt for the given obj based on the given startTime.
func (m Metrics) RecordDuration(ctx context.Context, obj conditions.Getter, startTime time.Time) {
	if m.MetricsRecorder != nil {
		ref, err := reference.GetReference(m.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record duration")
			return
		}
		m.MetricsRecorder.RecordDuration(*ref, startTime)
	}
}

// RecordSuspend records the suspension of the given obj based on the given suspend value.
func (m Metrics) RecordSuspend(ctx context.Context, obj conditions.Getter, suspend bool) {
	if m.MetricsRecorder != nil {
		ref, err := reference.GetReference(m.Scheme, obj)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record suspend")
			return
		}
		m.MetricsRecorder.RecordSuspend(*ref, suspend)
	}
}

// RecordReadiness records the flux_utils.ReadyCondition status for the given obj.
func (m Metrics) RecordReadiness(ctx context.Context, obj conditions.Getter) {
	m.RecordCondition(ctx, obj, meta.ReadyCondition)
}

// RecordReconciling records the flux_utils.ReconcilingCondition status for the given obj.
func (m Metrics) RecordReconciling(ctx context.Context, obj conditions.Getter) {
	m.RecordCondition(ctx, obj, meta.ReconcilingCondition)
}

// RecordStalled records the flux_utils.StalledCondition status for the given obj.
func (m Metrics) RecordStalled(ctx context.Context, obj conditions.Getter) {
	m.RecordCondition(ctx, obj, meta.StalledCondition)
}

// RecordCondition records the status of the given conditionType for the given obj.
func (m Metrics) RecordCondition(ctx context.Context, obj conditions.Getter, conditionType string) {
	if m.MetricsRecorder == nil {
		return
	}
	ref, err := reference.GetReference(m.Scheme, obj)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Error(err, "unable to get object reference to record condition metric")
		return
	}
	rc := conditions.Get(obj, conditionType)
	if rc == nil {
		rc = conditions.UnknownCondition(conditionType, "", "")
	}
	m.MetricsRecorder.RecordCondition(*ref, *rc, !obj.GetDeletionTimestamp().IsZero())
}
