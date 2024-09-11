/*
Copyright 2024.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OpsyConditionType string

const (
	ConditionTypeAMIUpdated           OpsyConditionType = "AMIUpdated"
	ConditionTypeAMICreated           OpsyConditionType = "AMICreated"
	ConditionTypeAMIUpdateFailed      OpsyConditionType = "AMIUpdateFailed"
	ConditionTypeAMIUpdateStarted     OpsyConditionType = "AMIUpdateStarted"
	ConditionTypeAMIUpdateSucceeded   OpsyConditionType = "AMIUpdateSucceeded"
	ConditionTypeBuildFailed          OpsyConditionType = "BuildFailed"
	ConditionTypeBuildStarted         OpsyConditionType = "BuildStarted"
	ConditionTypeBuildSucceeded       OpsyConditionType = "BuildSucceeded"
	ConditionTypeCancelled            OpsyConditionType = "Cancelled"
	ConditionTypeInitial              OpsyConditionType = "Initial"
	ConditionTypeCompleted            OpsyConditionType = "Completed"
	ConditionTypeFailed               OpsyConditionType = "Failed"
	ConditionTypeFlightCheckFailed    OpsyConditionType = "FlightCheckFailed"
	ConditionTypeFlightCheckPassed    OpsyConditionType = "FlightCheckPassed"
	ConditionTypeJobCompleted         OpsyConditionType = "JobCompleted"
	ConditionTypeInProgress           OpsyConditionType = "InProgress"
	ConditionTypeJobCreated           OpsyConditionType = "JobCreated"
	ConditionTypeJobFailed            OpsyConditionType = "JobFailed"
	ConditionTypeMaxFailedJobsReached OpsyConditionType = "MaxFailedJobsReached"
	ConditionTypeFlightCheckStarted   OpsyConditionType = "FlightCheckStarted"
)

type OpsyCondition struct {
	Type               OpsyConditionType      `json:"type"`
	Status             corev1.ConditionStatus `json:"status"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
}

func (c OpsyCondition) GetType() OpsyConditionType           { return c.Type }
func (c OpsyCondition) GetStatus() corev1.ConditionStatus    { return c.Status }
func (c OpsyCondition) GetLastTransitionTime() metav1.Time   { return c.LastTransitionTime }
func (c OpsyCondition) GetReason() string                    { return c.Reason }
func (c OpsyCondition) GetMessage() string                   { return c.Message }
func (c *OpsyCondition) SetStatus(s corev1.ConditionStatus)  { c.Status = s }
func (c *OpsyCondition) SetLastTransitionTime(t metav1.Time) { c.LastTransitionTime = t }
func (c *OpsyCondition) SetReason(r string)                  { c.Reason = r }
func (c *OpsyCondition) SetMessage(m string)                 { c.Message = m }
func (c *OpsyCondition) SetType(t OpsyConditionType)         { c.Type = t }
