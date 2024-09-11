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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AMIRefresherSpec defines the desired state of AMIRefresher
type AMIRefresherSpec struct {
	//+required
	Region string `json:"region"`
	//+optional
	AMI string `json:"ami,omitempty"`
	//+optional
	AMIFilters []AMIFilters `json:"amiFilters,omitempty"`
	//+required
	ClusterName string `json:"clusterName"`
	//+optional
	Exclude []string `json:"exclude,omitempty"`
	//+optional
	TimeOuts TimeOuts `json:"timeOuts,omitempty"`

	//+optional
	RefreshConfig RefreshConfig `json:"refreshConfig,omitempty"`
}

type RefreshConfig struct {
	Enable                   bool   `json:"enable,omitempty"`
	Force                    bool   `json:"force,omitempty"`
	MaxUnavailable           *int32 `json:"maxUnavailable,omitempty"`
	MaxUnavailablePercentage *int32 `json:"maxUnavailablePercentage,omitempty"`
	WaitForCompletion        bool   `json:"waitForCompletion,omitempty"`
}

// AMIRefresherStatus defines the observed state of AMIRefresher
type AMIRefresherStatus struct {
	Conditions    []*OpsyCondition `json:"conditions,omitempty"`
	LastRun       metav1.Time      `json:"lastRun,omitempty"`
	LastRunStatus LastRunStatus    `json:"lastRunStatus,omitempty"`
}

func (ar *AMIRefresher) SetConditions(conditions []*OpsyCondition) {
	ar.Status.Conditions = conditions
}

func (ar *AMIRefresher) GetConditions() []*OpsyCondition {
	return ar.Status.Conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AMIRefresher is the Schema for the amirefreshers API
type AMIRefresher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AMIRefresherSpec   `json:"spec,omitempty"`
	Status AMIRefresherStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AMIRefresherList contains a list of AMIRefresher
type AMIRefresherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AMIRefresher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AMIRefresher{}, &AMIRefresherList{})
}
