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
	"errors"
	"fmt"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	"github.com/ibeify/opsy-ami-operator/pkg/cond"
	"github.com/ibeify/opsy-ami-operator/pkg/events"
	f "github.com/ibeify/opsy-ami-operator/pkg/finalizer"
	packermanager "github.com/ibeify/opsy-ami-operator/pkg/manager_packer"
	"github.com/ibeify/opsy-ami-operator/pkg/opsy"
	"github.com/ibeify/opsy-ami-operator/pkg/status"
)

type InitResult struct {
	Packer     *amiv1alpha1.PackerBuilder
	PM         *packermanager.ManagerPacker
	ValidFor   time.Duration
	RetryAfter time.Duration
}

type JobResult struct {
	Succeeded bool
	Failed    bool
	Reason    string
}

type BuildResult struct {
	Completed bool
	Succeeded bool
	Failed    bool
	Reason    string
}

// PackerBuilderReconciler reconciles a PackerBuilder object
type PackerBuilderReconciler struct {
	client.Client
	Clientset    kubernetes.Interface
	Cond         *cond.ConditionManager
	EventManager *events.EventManager
	Scheme       *runtime.Scheme
	OpsyRunner   *opsy.OpsyRunner
	Status       *status.Status
}

// +kubebuilder:rbac:groups=ami.opsy.dev,resources=packerbuilders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=packerbuilders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=packerbuilders/finalizers,verbs=update
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=packerbuilders/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=packerbuilders/finalizers,verbs=update
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ami.opsy.dev,resources=pods/log,verbs=get;list;watch

func (r *PackerBuilderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	coloredLog := log.FromContext(ctx)
	coloredLog.V(2).Info("New reconcilation cycle starts...", "name", req.Name, "namespace", req.Namespace)

	var pb amiv1alpha1.PackerBuilder
	if err := r.Get(ctx, req.NamespacedName, &pb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pb.DeletionTimestamp != nil {
		return reconcile.Result{}, f.HandleFinalizer(ctx, &pb, r.Client, coloredLog)
	}

	if pb.Status.BuildID == "" {
		pb.Status.BuildID = uuid.New().String()
	}
	if pb.Status.Conditions == nil || len(pb.Status.Conditions) == 0 {
		r.Cond.SetCondition(ctx, &pb, amiv1alpha1.ConditionTypeInitial, corev1.ConditionTrue, "Starting", "Transitioning to Initializing")
	}

	if err := f.HandleFinalizer(ctx, &pb, r.Client, coloredLog); err != nil {
		return ctrl.Result{}, err
	}

	if pb.Status.State == "" {
		r.OpsyRunner.DefaultState(pb.Status.BuildID)
		if err := r.Status.Update(ctx, &pb); err != nil {

			if errors.Is(err, context.DeadlineExceeded) {
				coloredLog.Info("Context deadline exceeded while updating status")
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
	}
	return r.catchRek(ctx, &pb, coloredLog)
}

func (r *PackerBuilderReconciler) catchRek(ctx context.Context, pb *amiv1alpha1.PackerBuilder, log logr.Logger) (ctrl.Result, error) {
	pm := packermanager.New(ctx, r.Client, log, r.Scheme, r.Clientset.(*kubernetes.Clientset), pb)

	if r.OpsyRunner.GetCurrentState(pb.Status.BuildID) != pb.Status.State {
		log.V(1).Info("State mismatch, updating state", "opsyrunnerstate", r.OpsyRunner.GetCurrentState(pb.Status.BuildID), "controllerstate", pb.Status.State, "buildid", pb.Status.BuildID)
		if err := r.OpsyRunner.SetState(pb.Status.BuildID, pb.Status.State); err != nil {
			return r.transitionToErrorState(ctx, pb, fmt.Sprintf("failed to update state: %v", err), log)
		}
	}

	log.V(1).Info(fmt.Sprintf("Current State: %s", pb.Status.State))

	switch r.OpsyRunner.GetCurrentState(pb.Status.BuildID) {
	case amiv1alpha1.StateInitial:
		return r.init(ctx, pb, log)
	case amiv1alpha1.StateFlightChecks:
		return r.flightCheck(ctx, pb, pm, log)
	case amiv1alpha1.StateJobCreation:
		return r.createBuild(ctx, pb, pm, log)
	case amiv1alpha1.StateJobRunning:
		return r.runningBuild(ctx, pb, pm, log)
	case amiv1alpha1.StateJobCompleted:
		return r.completeBuild(ctx, pb, pm, log)
	case amiv1alpha1.StateAMICreated:
		return r.cleanupBuild(ctx, pb, pm, log)
	case amiv1alpha1.StateError:
		return r.errorBuild(ctx, pb, pm, log)
	default:
		log.Error(nil, "Unknown state encountered", "state", r.OpsyRunner.GetCurrentState(pb.Status.BuildID))
		return ctrl.Result{}, fmt.Errorf("unknown state: %s", r.OpsyRunner.GetCurrentState(pb.Status.BuildID))
	}
}

func (r *PackerBuilderReconciler) init(ctx context.Context, pb *amiv1alpha1.PackerBuilder, log logr.Logger) (ctrl.Result, error) {
	log.Info("Starting Init", "resource", pb.Name)
	if pb.Spec.MaxNumberOfJobs == nil {
		defaultMaxJobs := int32(3)
		pb.Spec.MaxNumberOfJobs = &defaultMaxJobs
	}

	if pb.Status.FailedJobCount == nil {
		pb.Status.FailedJobCount = new(int32)
	}

	if *pb.Status.FailedJobCount >= *pb.Spec.MaxNumberOfJobs {
		log.Info("Maximum number of failed jobs reached", "FailedJobCount", *pb.Status.FailedJobCount)
		retryAfter, _ := time.ParseDuration(pb.Spec.TimeOuts.ControllerTimer)
		return ctrl.Result{RequeueAfter: retryAfter}, nil
	}

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateFlightChecks, "Transitioning to FlightChecks", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeFlightCheckStarted, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to FlightChecks state: %s", err), log)
	}

	log.Info("Successfully handled Initialize state, transitioning to FlightChecks")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) flightCheck(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {

	log.Info("Starting Flight Checks", "resource", pb.Name)

	activeBuild := pm.ActiveBuildCheck(ctx, pb.Namespace, pb.Name)

	validFor, _ := time.ParseDuration(pb.Spec.TimeOuts.ExpiresIn)
	retryAfter, _ := time.ParseDuration(pb.Spec.TimeOuts.ControllerTimer)

	log.V(1).Info(activeBuild.Message)

	if !activeBuild.Success {

		if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobRunning, "Transitioning to JobRunning since Active Build Exist", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeFlightCheckPassed, log); err != nil {
			return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to JobRunning state: %s", err), log)
		}
		log.Info(activeBuild.Message)
		return ctrl.Result{Requeue: true}, nil
	}

	flightChecksPassed, err := r.performFlightChecks(ctx, pb, validFor, log)
	if err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Error performing flight checks: %s", err), log)
	}

	if !flightChecksPassed {
		log.Info(fmt.Sprintf("Requeuing after %s", retryAfter))
		return ctrl.Result{RequeueAfter: retryAfter}, nil
	}

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobCreation, "Transitioning to JobCreation", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeJobCreated, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to JobCreation state: %s", err), log)
	}

	log.Info("Successfully handled FlightChecks state, transitioning to JobCreation")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) createBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	log.Info("Create New Build", "resource", pb.Name)

	if err := r.setupJob(ctx, pb, pm, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to setup job: %v", err), log)
	}

	// retryAfter, _ := time.ParseDuration(pb.Spec.TimeOuts.ControllerTimer)

	// if pb.Spec.GitSync.Secret == "" {
	// 	log.Info(fmt.Sprintf("No secret specified for Git sync... Requeue after %s", retryAfter))
	// 	return ctrl.Result{RequeueAfter: retryAfter}, nil
	// }

	amiBuild, err := pm.Job(ctx, pb)
	if err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Unexpected Job Status: %v", err), log)
	}

	if err := controllerutil.SetControllerReference(pb, amiBuild, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference on job")
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to set owner reference on job: %v", err), log)
	}

	if err = r.Get(ctx, client.ObjectKeyFromObject(pb), amiBuild); err != nil && k8serrors.IsNotFound(err) {
		if err := r.Create(ctx, amiBuild); err != nil {
			return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Unexpect Job Status %v", err), log)
		}
	}

	log.Info("Created Job", "JobName", amiBuild.Name)

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobRunning, "Transitioning to JobRunning", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeInProgress, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to JobRunning state: %v", err), log)
	}

	log.Info("Successfully handled JobCreation state, transitioning to JobRunning")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) cleanupBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {

	log.Info("Ensure build artifacts are cleaned up from AWS")

	// Delete KeyPair If it Exists
	if pb.Status.LastRunKeyPair != "" {
		if err := pm.DeleteKeyPair(ctx, pb.Status.LastRunKeyPair); err != nil {
			return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to delete KeyPair: %v", err), log)
		}
	}

	log.Info(fmt.Sprintf("Deleted KeyPair %s", pb.Status.LastRunKeyPair))
	pb.Status.LastRunKeyPair = ""

	// Delete SecurityGroup If it Exists
	if pb.Status.LastRunSecurityGroupID != "" {
		if err := pm.DeleteSecurityGroup(ctx, pb.Status.LastRunSecurityGroupID); err != nil {
			return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to delete Security Group: %v", err), log)
		}
	}

	log.Info(fmt.Sprintf("Deleted SecurityGroup %s", pb.Status.LastRunSecurityGroupID))
	pb.Status.LastRunSecurityGroupID = ""

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateInitial, "Transitioning to Intializing", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeInitial, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to Initialize state: %v", err), log)
	}

	log.Info("Successfully handled AMICreated state, transitioning to Initialize")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) completeBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	log.Info("Completed Build, Post processing", "resource", pb.Name)

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateAMICreated, "Transitioning to AMICreated", amiv1alpha1.LastRunStatusCompleted, corev1.ConditionTrue, amiv1alpha1.ConditionTypeAMICreated, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to AMICreated state: %v", err), log)
	}

	log.Info("Successfully handled JobCompleted state, transitioning to AMICreated")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) transitionToJobCompletedState(ctx context.Context, pb *amiv1alpha1.PackerBuilder, log logr.Logger) (ctrl.Result, error) {
	log.Info("transitionToJobCompletedState", "resource", pb.Name)

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobCompleted, "Transitioning to JobCompleted", amiv1alpha1.LastRunStatusCompleted, corev1.ConditionTrue, amiv1alpha1.ConditionTypeCompleted, log); err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to transition to JobCompleted state: %v", err), log)
	}

	log.Info("Successfully handled JobRunning state, transitioning to JobCompleted")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) runningBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	resultChan, errChan, cleanup, err := r.watchBuild(ctx, pb, pm, log)
	if err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to start build watch: %v", err), log)
	}
	defer cleanup()

	ticker := time.NewTicker(15 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case result := <-resultChan:
			if result.Completed {
				if result.Succeeded {
					log.Info("Job completed successfully")
					return r.transitionToJobCompletedState(ctx, pb, log)
				} else {
					log.Error(nil, "Job failed", "reason", result.Reason)
					return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Job failed: %s", result.Reason), log)
				}
			}

		case err := <-errChan:
			log.Error(err, "Error during build watch")
			return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Build watch error: %v", err), log)

		case <-ticker.C:

			// Check if PackerBuilder still exists
			if err := r.Get(ctx, types.NamespacedName{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
				if k8serrors.IsNotFound(err) {
					log.Info("PackerBuilder resource was deleted, stopping operations")
					return ctrl.Result{}, nil

				}
				log.Error(err, "Failed to get PackerBuilder resource")
				return r.transitionToErrorState(ctx, pb, "Failed to verify PackerBuilder existence", log)
			}

			// Check if Job still exists
			job := &batchv1.Job{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: pb.Namespace, Name: pb.Status.JobName}, job); err != nil {
				if k8serrors.IsNotFound(err) {
					log.Info("Job was deleted, transitioning to error state")
					return r.transitionToErrorState(ctx, pb, "Associated Job was deleted", log)
				}
				log.Error(err, "Failed to get Job resource")
				return r.transitionToErrorState(ctx, pb, "Failed to verify Job existence", log)
			}

		case <-ctx.Done():
			log.Info("Context cancelled, stopping build watch")
			return ctrl.Result{}, nil
		}
	}
}

func (r *PackerBuilderReconciler) watchBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (<-chan BuildResult, <-chan error, func(), error) {
	log.Info("Watch Build Started", "resource", pb.Name)

	resultChan := make(chan BuildResult, 1)
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		r.watchJobCompletion(ctx, pb.Namespace, pb.Status.JobName, resultChan, errChan)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := r.streamPodLogs(ctx, pb, log)
		if err != nil && err != context.Canceled {
			errChan <- fmt.Errorf("error streaming pod logs: %w", err)
		}
	}()

	cleanup := func() {
		cancel()

		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.V(1).Info("All goroutines completed gracefully")
		case <-cleanupCtx.Done():
			log.V(1).Info("Cleanup timed out, some goroutines may not have completed")
		}

		go func() {
			for range resultChan {
			}
		}()
		go func() {
			for range errChan {
			}
		}()

		close(resultChan)
		close(errChan)
	}

	return resultChan, errChan, cleanup, nil
}

func (r *PackerBuilderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amiv1alpha1.PackerBuilder{}).
		Owns(&batchv1.Job{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
}
