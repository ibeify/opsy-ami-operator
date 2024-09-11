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

package cond

import (
	"context"
	"sort"

	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	"github.com/ibeify/opsy-ami-operator/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConditionSetter interface {
	client.Object
	GetConditions() []*amiv1alpha1.OpsyCondition
	SetConditions([]*amiv1alpha1.OpsyCondition)
}

type ConditionManager struct {
	client client.Client
}

func New(c client.Client) *ConditionManager {
	return &ConditionManager{client: c}
}

func (cm *ConditionManager) SetCondition(ctx context.Context, obj ConditionSetter, conditionType amiv1alpha1.OpsyConditionType, condStatus corev1.ConditionStatus, reason, message string) error {
	currentTime := metav1.Now()
	conditions := obj.GetConditions()
	existingCondition := false
	for i := range conditions {
		condition := &conditions[i]
		if (*condition).Type == conditionType {
			if (*condition).Status != condStatus || (*condition).Reason != reason || (*condition).Message != message {
				(*condition).Status = condStatus
				(*condition).LastTransitionTime = currentTime
				(*condition).Reason = reason
				(*condition).Message = message
			}
			existingCondition = true
			break
		}
	}
	if !existingCondition {
		newCondition := amiv1alpha1.OpsyCondition{
			Type:               conditionType,
			Status:             condStatus,
			LastTransitionTime: currentTime,
			Reason:             reason,
			Message:            message,
		}
		conditions = append(conditions, &newCondition)
	}
	sort.Slice(conditions, func(i, j int) bool {
		return conditions[i].LastTransitionTime.After(conditions[j].LastTransitionTime.Time)
	})
	obj.SetConditions(conditions)
	statusUpdater := status.New(cm.client)
	return statusUpdater.Update(ctx, obj)
}
