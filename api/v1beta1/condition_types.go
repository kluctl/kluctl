/*
Copyright 2023.

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

package v1beta1

const (
	// DiffFailedReason represents the fact that the
	// kluctl diff command failed.
	DiffFailedReason string = "DiffFailed"

	// DeployFailedReason represents the fact that the
	// kluctl deploy command failed.
	DeployFailedReason string = "DeployFailed"

	// PruneFailedReason represents the fact that the
	// kluctl prune command failed.
	PruneFailedReason string = "PruneFailed"

	// ValidateFailedReason represents the fact that the
	// validate of the KluctlDeployment failed.
	ValidateFailedReason string = "ValidateFailed"

	// PrepareFailedReason represents failure in the kluctl preparation phase
	PrepareFailedReason string = "PrepareFailed"

	// ReconciliationSucceededReason represents the fact that
	// the reconciliation succeeded.
	ReconciliationSucceededReason string = "ReconciliationSucceeded"

	// WaitingForLegacyMigrationReason means that the controller is waiting for the legacy controller to set `readyForMigration=true`
	WaitingForLegacyMigrationReason string = "WaitingForLegacyMigration"
)
