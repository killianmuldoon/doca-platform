/*
Copyright 2024 NVIDIA

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

package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConditionType represents different types of conditions in the conditions pkg.
// There are generally three types of conditions:
//
//   - Ready: A singleton type indicating the overall status of the controller.
//     Possible reasons include: Success, Failure, Pending, or AwaitingDeletion.
//
//   - XXXReady: Indicates the readiness status of specific resources managed by the controller.
//     Possible reasons include: Success, Failure, Pending, or AwaitingDeletion.
//
//   - XXXReconciled: Reflects the status of the reconciliation process, indicating that
//     changes to the resource have been applied.
//     Possible reasons include: Success, Error, Pending, or AwaitingDeletion.
type ConditionType string

const (
	// TypeReady is the overall ready type for the controller.
	TypeReady ConditionType = "Ready"
)

// ConditionReason is the type for the reason of a condition.
type ConditionReason string

const (
	// ReasonAwaitingDeletion if the controller is waiting for the deletion.
	ReasonAwaitingDeletion ConditionReason = "AwaitingDeletion"
	// ReasonPending indicates that the resource has not yet reached the expected state.
	ReasonPending ConditionReason = "Pending"
	// ReasonError is an error the system CAN recover from.
	ReasonError ConditionReason = "Error"
	// ReasonFailure is a terminal state the system CANNOT recover from.
	ReasonFailure ConditionReason = "Failure"
	// ReasonSuccess is the success reason.
	ReasonSuccess ConditionReason = "Success"
)

// ConditionMessage is the message type for the conditions.
type ConditionMessage string

const (
	// MessageSuccess is the default success message.
	MessageSuccess ConditionMessage = "Reconciliation successful"
)

type GetSet interface {
	GetConditions() []metav1.Condition
	SetConditions([]metav1.Condition)
}

// EnsureConditions ensures that all specified conditions are present.
// allConditions can be left nil if no conditions must be initialized.
func EnsureConditions(obj client.Object, allConditions []ConditionType) {
	mutateObj := obj.(GetSet)
	conditions := GetSet.GetConditions(mutateObj)

	if conditions == nil {
		conditions = []metav1.Condition{}
	}

	// Ensure all conditions exist.
	for _, condition := range allConditions {
		if meta.FindStatusCondition(conditions, string(condition)) == nil {
			meta.SetStatusCondition(&conditions, metav1.Condition{
				Type:    string(condition),
				Status:  metav1.ConditionUnknown,
				Reason:  string(ReasonPending),
				Message: "",
			})
		}
	}

	mutateObj.SetConditions(conditions)
}

// AddTrue adds a condition with Status=True, Reason=Successful and Message=Reconciliation successful.
func AddTrue(obj client.Object, conditionType ConditionType) {
	add(obj, metav1.ConditionTrue, conditionType, ReasonSuccess, MessageSuccess)
}

// AddFalse adds a condition with Status=False, Reason=Pending and a specified message.
func AddFalse(obj client.Object, conditionType ConditionType, conditionReason ConditionReason, conditionMessage ConditionMessage) {
	add(obj, metav1.ConditionFalse, conditionType, conditionReason, conditionMessage)
}

func add(obj client.Object, cs metav1.ConditionStatus, ct ConditionType, cr ConditionReason, cm ConditionMessage) {
	mutateObj := obj.(GetSet)
	conditions := GetSet.GetConditions(mutateObj)

	if conditions == nil {
		conditions = []metav1.Condition{}
	}

	meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:    string(ct),
		Status:  cs,
		Reason:  string(cr),
		Message: string(cm),
	})

	mutateObj.SetConditions(conditions)
}

// Get returns a condition with a specific type.
func Get(obj client.Object, conditionType ConditionType) *metav1.Condition {
	mutateObj := obj.(GetSet)
	conditions := GetSet.GetConditions(mutateObj)

	for _, c := range conditions {
		if c.Type == string(conditionType) {
			return &c
		}
	}
	return nil
}
