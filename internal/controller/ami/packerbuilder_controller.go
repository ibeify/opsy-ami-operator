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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	var pb amiv1alpha1.PackerBuilder
	if err := r.Get(ctx, req.NamespacedName, &pb); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pb.Status.BuildID == "" {
		pb.Status.BuildID = uuid.New().String()
	}
	if pb.Status.Conditions == nil || len(pb.Status.Conditions) == 0 {
		r.Cond.SetCondition(ctx, &pb, amiv1alpha1.ConditionTypeInitial, corev1.ConditionTrue, "Starting", "Transitioning to Initializing")
		if err := r.Status.Update(ctx, &pb); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				coloredLog.Info("Context deadline exceeded while updating status")
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, &pb); err != nil {
			return ctrl.Result{}, err
		}
	}

	if pb.DeletionTimestamp != nil {
		return reconcile.Result{}, f.HandleFinalizer(ctx, &pb, r.Client, coloredLog)
	}

	if err := f.HandleFinalizer(ctx, &pb, r.Client, coloredLog); err != nil {
		return ctrl.Result{}, err
	}

	if r.shouldResetState(&pb) {
		return r.resetState(ctx, &pb, coloredLog)
	}

	if pb.Status.State == "" {
		pb.Status.State = r.OpsyRunner.GetCurrentState(pb.Status.BuildID)
		if err := r.Status.Update(ctx, &pb); err != nil {

			if errors.Is(err, context.DeadlineExceeded) {
				coloredLog.Info("Context deadline exceeded while updating status")
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
	}
	return r.catchRec(ctx, &pb, coloredLog)
}

func (r *PackerBuilderReconciler) catchRec(ctx context.Context, pb *amiv1alpha1.PackerBuilder, log logr.Logger) (ctrl.Result, error) {
	pm := packermanager.New(ctx, r.Client, log, r.Scheme, r.Clientset.(*kubernetes.Clientset), pb)

	log.Info(fmt.Sprintf("Current State: %s", pb.Status.State))
	if r.OpsyRunner.GetCurrentState(pb.Status.BuildID) != pb.Status.State {
		if err := r.OpsyRunner.SetState(pb.Status.BuildID, pb.Status.State); err != nil {
			return ctrl.Result{}, err
		}
	}

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

func (r *PackerBuilderReconciler) resetState(ctx context.Context, pb *amiv1alpha1.PackerBuilder, log logr.Logger) (ctrl.Result, error) {
	pb.Status.State = amiv1alpha1.StateInitial
	pb.Status.LastTransitionTime = metav1.Now()

	// Clear any error conditions or other status fields as needed
	pb.Status.Conditions = []*amiv1alpha1.OpsyCondition{}

	if err := r.Status.Update(ctx, pb); err != nil {
		log.Error(err, "Failed to reset state")
		return ctrl.Result{}, err
	}

	log.Info("State reset to initial", "resource", pb.Name)
	return ctrl.Result{Requeue: true}, nil
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
		return r.transitionToErrorState(ctx, pb, "Failed to transition to FlightChecks state", log)
	}

	log.Info("Successfully handled Initialize state, transitioning to FlightChecks")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) flightCheck(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	log.Info("Starting Flight Checks", "resource", pb.Name)
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		return r.transitionToErrorState(ctx, pb, "Failed to get PackerBuilder", log)
	}

	activeBuild := pm.ActiveBuildCheck(ctx, pb.Namespace, pb.Name)

	validFor, _ := time.ParseDuration(pb.Spec.TimeOuts.ExpiresIn)
	retryAfter, _ := time.ParseDuration(pb.Spec.TimeOuts.ControllerTimer)

	log.V(1).Info(activeBuild.Message)
	pb.Spec.Builder.ImageType = "ami"

	if !activeBuild.Success {
		log.Info(activeBuild.Message)

		if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobRunning, "Transitioning to JobRunning since Active Build Exist", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeFlightCheckPassed, log); err != nil {
			return r.transitionToErrorState(ctx, pb, "Failed to transition to JobRunning state", log)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	flightChecksPassed, err := r.performFlightChecks(ctx, pb, validFor, log)
	if err != nil {
		return r.transitionToErrorState(ctx, pb, "Error performing flight checks", log)
	}

	if !flightChecksPassed {
		log.Info(fmt.Sprintf("Requeuing after %s", retryAfter))
		return ctrl.Result{RequeueAfter: retryAfter}, nil
	}

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobCreation, "Transitioning to JobCreation", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeJobCreated, log); err != nil {
		return r.transitionToErrorState(ctx, pb, "Failed to transition to JobCreation state", log)
	}

	log.Info("Successfully handled FlightChecks state, transitioning to JobCreation")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) createBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	log.Info("Create New Build", "resource", pb.Name)
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		log.Error(err, "Failed to get PackerBuilder")
		return ctrl.Result{}, err
	}

	if err := r.setupJob(ctx, pb, pm, log); err != nil {
		log.Error(err, "Failed to setup job")
		return r.transitionToErrorState(ctx, pb, "Failed to setup job", log)
	}

	retryAfter, _ := time.ParseDuration(pb.Spec.TimeOuts.ControllerTimer)

	if pb.Spec.GitSync.Secret == "" {
		log.Info(fmt.Sprintf("No secret specified for Git sync... Requeue after %s", retryAfter))
		return ctrl.Result{RequeueAfter: retryAfter}, nil
	}

	amiBuild, err := pm.Job(ctx, *pb)
	if err != nil {
		return r.transitionToErrorState(ctx, pb, "Unexpected job status", log)
	}

	if err := controllerutil.SetControllerReference(pb, amiBuild, r.Scheme); err != nil {
		log.Error(err, "Failed to set owner reference on job")
		return r.transitionToErrorState(ctx, pb, "Failed to set owner reference on job", log)
	}

	if err = r.Get(ctx, client.ObjectKeyFromObject(pb), amiBuild); err != nil && k8serrors.IsNotFound(err) {
		if err := r.Create(ctx, amiBuild); err != nil {
			return r.transitionToErrorState(ctx, pb, "Unexpected job status", log)
		}
	}

	log.Info("Created Job", "JobName", amiBuild.Name)

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateJobRunning, "Transitioning to JobRunning", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeInProgress, log); err != nil {
		log.Error(err, "Failed to transition to JobRunning state")
		return r.transitionToErrorState(ctx, pb, "Failed to transition to JobRunning state", log)
	}

	log.Info("Successfully handled JobCreation state, transitioning to JobRunning")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) completeBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	log.Info("Completed Build, Post processing", "resource", pb.Name)
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		log.Error(err, "Failed to get PackerBuilder")
		return ctrl.Result{}, err
	}
	// Do nothing for not but update the state

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateAMICreated, "Transitioning to AMIReady", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeAMICreated, log); err != nil {
		log.Error(err, "Failed to transition to AMIReady state")
		return r.transitionToErrorState(ctx, pb, "Unexpected job status", log)
	}

	log.Info("Successfully handled JobCompleted state, transitioning to AMIReady")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) cleanupBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {

	log.Info("Ensure build artifacts are cleaned up from AWS")
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		log.Error(err, "Failed to get PackerBuilder")
		return ctrl.Result{}, err
	}

	// Delete KeyPair If it Exists
	if pb.Status.LastRunKeyPair != "" {
		if err := pm.DeleteKeyPair(ctx, pb.Status.LastRunKeyPair); err != nil {
			log.Error(err, "Failed to delete KeyPair", "KeyPair", pb.Status.LastRunKeyPair)
			return r.transitionToErrorState(ctx, pb, "Failed to delete KeyPair", log)
		}
	}

	log.Info(fmt.Sprintf("Deleted KeyPair %s", pb.Status.LastRunKeyPair))

	// Delete SecurityGroup If it Exists
	if pb.Status.LastRunSecurityGroupID != "" {
		if err := pm.DeleteSecurityGroup(ctx, pb.Status.LastRunSecurityGroupID); err != nil {
			log.Error(err, "Failed to delete SecurityGroup", "SecurityGroup", pb.Status.LastRunSecurityGroupID)
			return r.transitionToErrorState(ctx, pb, "Failed to delete SecurityGroup", log)
		}
	}

	log.Info(fmt.Sprintf("Deleted SecurityGroup %s", pb.Status.LastRunSecurityGroupID))

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateInitial, "Transitioning to Intializing", amiv1alpha1.LastRunStatusRunning, corev1.ConditionTrue, amiv1alpha1.ConditionTypeInitial, log); err != nil {
		log.Error(err, "Failed to transition to Initial state")
		return r.transitionToErrorState(ctx, pb, "Failed to transition to Initial state", log)
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) errorBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		log.Error(err, "Failed to get PackerBuilder")
		return ctrl.Result{}, err
	}
	// Leave empty for now
	// Should check how many previously failed attempts to be  build the AMI
	// Close all connections and clean up any lingering resources

	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateInitial, "Transitioning to Initializing", amiv1alpha1.LastRunStatusFailed, corev1.ConditionTrue, amiv1alpha1.ConditionTypeInitial, log); err != nil {
		log.Error(err, "Failed to transition to Error state")
		return r.transitionToErrorState(ctx, pb, "Failed to transition to Initial state", log)
	}

	log.Info("Successfully handled Error state, transitioning to Initializing")
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) transitionToJobCompletedState(ctx context.Context, pb *amiv1alpha1.PackerBuilder, log logr.Logger) (ctrl.Result, error) {
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		log.Error(err, "Failed to get PackerBuilder")
		return ctrl.Result{}, err
	}

	pb.Status.State = amiv1alpha1.StateJobCompleted
	if err := r.Status.Update(ctx, pb); err != nil {
		log.Error(err, "Failed to update state to JobCompleted")
		return r.transitionToErrorState(ctx, pb, "Failed to update state to JobCompleted", log)
	}

	if err := r.OpsyRunner.SetState(pb.Status.BuildID, amiv1alpha1.StateJobCompleted); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) watchBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (<-chan string, <-chan BuildResult, <-chan error, func(), error) {
	log.Info("Watch Build Started", "resource", pb.Name)
	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		log.Error(err, "Failed to get PackerBuilder")
		return nil, nil, nil, nil, err
	}

	logChan := make(chan string, 100)
	resultChan := make(chan BuildResult, 1)
	errChan := make(chan error, 2)

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
		err := r.streamPodLogs(ctx, pb, logChan, log)
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
			for range logChan {
			}
		}()
		go func() {
			for range resultChan {
			}
		}()
		go func() {
			for range errChan {
			}
		}()

		close(logChan)
		close(resultChan)
		close(errChan)
	}

	return logChan, resultChan, errChan, cleanup, nil
}

func (r *PackerBuilderReconciler) shouldResetState(pb *amiv1alpha1.PackerBuilder) bool {

	// 1. If the last reconciliation failed
	// 2. If a certain amount of time has passed since the last update
	// 3. If there's a specific annotation indicating a reset is needed

	if pb.Status.State == amiv1alpha1.StateError {
		return true
	}

	if !pb.Status.LastTransitionTime.IsZero() {
		if time.Since(pb.Status.LastTransitionTime.Time) > 1*time.Hour {
			return true
		}
	}

	if val, exists := pb.Annotations["reset-state"]; exists && val == "true" {
		return true
	}

	return false
}

func (r *PackerBuilderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&amiv1alpha1.PackerBuilder{}).
		Owns(&batchv1.Job{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 10}).
		Complete(r)
}
