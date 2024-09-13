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

package ami

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	"github.com/ibeify/opsy-ami-operator/pkg/cond"
	"github.com/ibeify/opsy-ami-operator/pkg/events"
	f "github.com/ibeify/opsy-ami-operator/pkg/finalizer"
	amimanager "github.com/ibeify/opsy-ami-operator/pkg/manager_ami"
	eksmanger "github.com/ibeify/opsy-ami-operator/pkg/manager_eks"
	"github.com/ibeify/opsy-ami-operator/pkg/status"
	corev1 "k8s.io/api/core/v1"
)

// AMIRefresherReconciler reconciles a AMIRefresher object
type AMIRefresherReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Clientset    kubernetes.Interface
	Status       *status.Status
	EventManager *events.EventManager
	Cond         *cond.ConditionManager
}

//+kubebuilder:rbac:groups=ami.opsy.dev,resources=amirefreshers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ami.opsy.dev,resources=amirefreshers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ami.opsy.dev,resources=amirefreshers/finalizers,verbs=update

func (r *AMIRefresherReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var ar amiv1alpha1.AMIRefresher
	if err := r.Get(ctx, req.NamespacedName, &ar); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ar.Status.Conditions == nil || len(ar.Status.Conditions) == 0 {

		err := r.setStatusCondition(ctx, &ar, amiv1alpha1.ConditionTypeInProgress, corev1.ConditionUnknown, "Reconciling", "Starting reconciliation", log)
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, &ar); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	if err := f.HandleFinalizer(ctx, &ar, r.Client, log); err != nil {
		log.Error(err, "Failed to handle finalizer")
		return ctrl.Result{}, err
	}

	isAMIRefresherMarkedForDeletion := ar.GetDeletionTimestamp() != nil
	if isAMIRefresherMarkedForDeletion {

		if err := r.setStatusCondition(ctx, &ar, amiv1alpha1.ConditionTypeInProgress, corev1.ConditionTrue, "Finalizing", "Finalizer operations for custom resource %s name were successfully accomplished", log); err != nil {
			log.Error(err, "Failed to set condition")
			return ctrl.Result{}, err
		}

		if err := f.HandleFinalizer(ctx, &ar, r.Client, log); err != nil {
			log.Error(err, "Failed to finalize")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, &ar); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	am := amimanager.New(ctx, ar.Spec.Region, log)
	ek8s := eksmanger.New(ctx, r.Client, ar.Spec.Region, ar.Spec.ClusterName, log)

	expiresIn, _ := time.ParseDuration(ar.Spec.TimeOuts.ExpiresIn)
	retryAfter, _ := time.ParseDuration(ar.Spec.TimeOuts.ControllerTimer)

	// If there are no AMI filters, then we expect the user has provide the AMI ID directly to be used for refresh
	if len(ar.Spec.AMIFilters) != 0 {

		results := am.AMIChecks(ctx, nil, expiresIn, amimanager.AMIRefreshCheck)
		if results.Error != nil {
			if err := r.setStatusCondition(ctx, &ar, amiv1alpha1.ConditionTypeFailed, corev1.ConditionTrue, "Failed", results.Error.Error(), log); err != nil {
				log.Error(err, "Failed to set condition")
				return ctrl.Result{}, err
			}
			log.Error(results.Error, "Failed to check for AMIs")
			return ctrl.Result{}, results.Error
		}
		ar.Spec.AMI = results.Name
	}

	updateConfig := eksmanger.UpdateConfig{
		WaitForCompletion: false,
	}

	nodeGroups, err := ek8s.ListNodeGroups(ctx, ar.Spec.ClusterName)
	if err != nil {

		if err := r.setStatusCondition(ctx, &ar, amiv1alpha1.ConditionTypeFailed, corev1.ConditionTrue, "Failed", err.Error(), log); err != nil {
			log.Error(err, "Failed to set condition")
			return ctrl.Result{}, err
		}
		log.Error(err, "Failed retrieve node groups")
		return ctrl.Result{}, err
	}

	trimNodeGroups := ek8s.TrimNodeGroups(nodeGroups.Nodegroups, ar.Spec.Exclude)

	for _, nodeGroupName := range trimNodeGroups {
		if err := ek8s.UpdateNodeGroupAMI(ctx, nodeGroupName, ar.Spec.AMI, updateConfig); err != nil {

			if err := r.setStatusCondition(ctx, &ar, amiv1alpha1.ConditionTypeFailed, corev1.ConditionTrue, "Failed", err.Error(), log); err != nil {
				log.Error(err, "Failed to set condition")
				return ctrl.Result{}, err
			}

			log.Error(err, "Failed to update EKS node group with new AMI")
			return ctrl.Result{}, err
		}
	}

	if err := r.setStatusCondition(ctx, &ar, amiv1alpha1.ConditionTypeAMIUpdated, corev1.ConditionTrue, "Completed", "Successfully Update AMI for all required node groups", log); err != nil {
		log.Error(err, "Failed to set condition")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: retryAfter}, nil

}

func (r *AMIRefresherReconciler) setStatusCondition(ctx context.Context, obj cond.ConditionSetter, conditionType amiv1alpha1.OpsyConditionType, condStatus corev1.ConditionStatus, reason string, message string, log logr.Logger) error {
	if err := r.Cond.SetCondition(ctx, obj, conditionType, condStatus, reason, message); err != nil {
		log.Error(err, "Failed to set condition")
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AMIRefresherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amiv1alpha1.AMIRefresher{}).
		Complete(r)
}
