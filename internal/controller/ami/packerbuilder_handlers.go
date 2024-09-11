package ami

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	amimanager "github.com/ibeify/opsy-ami-operator/pkg/manager_ami"
	packermanager "github.com/ibeify/opsy-ami-operator/pkg/manager_packer"
	"github.com/ibeify/opsy-ami-operator/pkg/notifier"
	"github.com/ibeify/opsy-ami-operator/pkg/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *PackerBuilderReconciler) processFlightCheckResults(results []status.Result, pb *amiv1alpha1.PackerBuilder, log logr.Logger) bool {
	allChecksSuccessful := true
	for _, result := range results {
		if !result.Success {
			log.Info(fmt.Sprintf("Flight check failed: %v", result.Message), "error", result.Error, "message", result.Message)
			allChecksSuccessful = false
		} else {
			if strings.Contains(result.Message, "Base AMI") {
				pb.Status.LastRunBaseImageID = result.Name
			}

			log.Info(fmt.Sprintf("Flight check passed: %v", result.Message), "message", result.Message)
		}
	}

	return allChecksSuccessful
}

func (r *PackerBuilderReconciler) performFlightChecks(ctx context.Context, pb *amiv1alpha1.PackerBuilder, validFor time.Duration, log logr.Logger) (bool, error) {
	var flightCheckResults []status.Result
	am := amimanager.New(ctx, pb.Spec.Region, log)
	pm := packermanager.New(ctx, r.Client, log, r.Scheme, r.Clientset.(*kubernetes.Clientset), pb)
	pb.Spec.Builder.ImageType = "ami"
	switch pb.Spec.Builder.ImageType {
	case amiv1alpha1.AMI:
		baseAMIResult := am.BaseAMIExists(ctx, pb.Spec.AMIFilters)
		flightCheckResults = append(flightCheckResults, baseAMIResult)

		latestAMIResult := pm.LatestAMIDoesNotExistsOrExpired(ctx, validFor, pb)
		flightCheckResults = append(flightCheckResults, latestAMIResult)
	case amiv1alpha1.Container:
		// Handle container case if needed
	}

	return r.processFlightCheckResults(flightCheckResults, pb, log), nil
}

func (r *PackerBuilderReconciler) transitionToErrorState(ctx context.Context, pb *amiv1alpha1.PackerBuilder, reason string, log logr.Logger) (ctrl.Result, error) {

	// Do nothing for now but update the state
	if err := r.setStateAndUpdate(ctx, pb, amiv1alpha1.StateError, reason, amiv1alpha1.LastRunStatusFailed, corev1.ConditionFalse, amiv1alpha1.ConditionTypeFailed, log); err != nil {
		log.Error(err, "Failed to transition to Error state")
		return ctrl.Result{}, err
	}

	log.Info("Transitioned to Error state", "reason", reason)
	return ctrl.Result{Requeue: true}, nil
}

func (r *PackerBuilderReconciler) setupJob(ctx context.Context, packer *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) error {

	packer.Status.JobID = uuid.New().String()[:5]
	packer.Status.JobName = fmt.Sprintf("%s-%s", packer.Name, packer.Status.JobID)
	packer.Status.JobStatus = amiv1alpha1.JobStatusRunning

	if err := pm.ConstructCommand(ctx, packer); err != nil {
		return err
	}
	log.V(1).Info("Constructed Packer Command", "Command", packer.Status.Command)

	if err := r.Status.Update(ctx, packer); err != nil {
		return err
	}

	return nil
}

func (r *PackerBuilderReconciler) setStateAndUpdate(ctx context.Context, pb *amiv1alpha1.PackerBuilder, newState amiv1alpha1.PackerBuilderState, message string, lastStatus amiv1alpha1.LastRunStatus, conditionStatus corev1.ConditionStatus, conditionType amiv1alpha1.OpsyConditionType, log logr.Logger) error {

	from := pb.Status.State

	r.EventManager.RecordNormalEvent(pb, string(newState), message)

	if err := r.Cond.SetCondition(ctx, pb, conditionType, conditionStatus, string(newState), message); err != nil {
		log.Error(err, "Failed to set condition")
		return err
	}

	log.Info("State transition successful", "from", pb.Status.State, "to", newState, "message", message)
	pb.Status.State = newState
	pb.Status.LastRunStatus = lastStatus
	pb.Status.LastRunMessage = message
	pb.Status.LastRun = metav1.Now()
	if err := r.Status.Update(ctx, pb); err != nil {
		log.Error(err, "Failed to update PackerBuilder status")
		return err
	}

	buildInfo := notifier.Message{
		Name:           pb.Name,
		LatestBaseAMI:  pb.Status.LastRunBaseImageID,
		LatestBuiltAMI: pb.Status.LastRunBuiltImageID,
		Message:        message,
	}
	switch pb.Status.State {
	case amiv1alpha1.StateInitial, amiv1alpha1.StateFlightChecks:
		// DO not send any message if Initialing or FlightChecks
	default:

		if !isEmpty(pb.Spec.Notify.Slack) {
			secret := corev1.Secret{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Spec.Notify.Slack.Secret}, &secret); err != nil {
				log.Error(err, "Failed to get secret")
				return err
			}
			// Extract the token from the secret
			tokenBytes, ok := secret.Data["token"]
			if !ok {
				log.Error(nil, "Secret does not contain 'token' key", "secretName", pb.Spec.Notify.Slack.Secret)
				return fmt.Errorf("secret does not contain 'token' key")
			}

			tokenString := string(tokenBytes)
			channels := strings.Join(pb.Spec.Notify.Slack.Channels, ", ")
			mess := notifier.NewSlackService(tokenString, channels)
			if err := mess.Send(ctx, pb.Name, buildInfo); err != nil {
				log.Error(err, "Failed to send slack message")
				return err
			}
		}
	}

	if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
		return err
	}

	if err := r.OpsyRunner.SetState(pb.Status.BuildID, newState); err != nil {
		log.Error(err, "Failed to transition state", "from", from, "to", newState)
		return err
	}

	return nil
}

func (r *PackerBuilderReconciler) streamPodLogs(ctx context.Context, pb *amiv1alpha1.PackerBuilder, logChan chan<- string, log logr.Logger) error {
	pm := packermanager.New(ctx, r.Client, log, r.Scheme, r.Clientset.(*kubernetes.Clientset), pb)

	return wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.InNamespace(pb.Namespace), client.MatchingLabels{"job-name": pb.Status.JobName}); err != nil {
			log.Error(err, "Failed to list pods for job")
			return false, nil // Retry
		}

		if len(podList.Items) == 0 {
			log.Info("No pods found for job, waiting...")
			return false, nil // Retry
		}

		pod := &podList.Items[0]

		// Check if the pod is still initializing
		if pod.Status.Phase == corev1.PodPending {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "PodInitializing" {
					log.Info("Pod is still initializing, waiting...", "pod", pod.Name)
					return false, nil // Retry
				}
			}
		}

		// Stream logs
		streamCtx, cancelStream := context.WithCancel(ctx)
		defer cancelStream()

		errChan := make(chan error, 1)
		go func() {
			err := pm.StreamLogs(streamCtx, pod.Name, pod.Namespace)
			errChan <- err
		}()

		select {
		case err := <-errChan:
			if err != nil {
				if strings.Contains(err.Error(), "PodInitializing") {
					log.Info("Pod is still initializing, will retry", "error", err)
					return false, nil // Retry
				}
				log.Error(err, "Error occurred during log streaming")
				return false, err // Stop retrying, propagate error
			}
			// StreamLogs completed successfully
			log.Info("Log streaming completed successfully")
			return true, nil // Stop polling, exit function
		case <-ctx.Done():
			log.Info("Context cancelled, stopping log streaming")
			return false, ctx.Err() // Stop retrying, context cancelled
		}
	})
}

func (r *PackerBuilderReconciler) getJobFailureReason(job *batchv1.Job) string {
	if len(job.Status.Conditions) > 0 {
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobFailed {
				return fmt.Sprintf("Job failed: %s", condition.Message)
			}
		}
	}
	return "Job failed for unknown reason"
}

func (r *PackerBuilderReconciler) runningBuild(ctx context.Context, pb *amiv1alpha1.PackerBuilder, pm *packermanager.ManagerPacker, log logr.Logger) (ctrl.Result, error) {
	logChan, resultChan, errChan, cleanup, err := r.watchBuild(ctx, pb, pm, log)
	if err != nil {
		return r.transitionToErrorState(ctx, pb, fmt.Sprintf("Failed to start build watch: %v", err), log)
	}
	defer cleanup()

	ticker := time.NewTicker(15 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case logMsg := <-logChan:
			log.Info("Received log message", "message", logMsg)
			// Update PackerBuilder status with the latest log message if needed

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
			if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Name}, pb); err != nil {
				if errors.IsNotFound(err) {
					log.Info("PackerBuilder resource was deleted, stopping operations")
					return ctrl.Result{}, nil

				}
				log.Error(err, "Failed to get PackerBuilder resource")
				return r.transitionToErrorState(ctx, pb, "Failed to verify PackerBuilder existence", log)
			}

			// Check if Job still exists
			job := &batchv1.Job{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: pb.Namespace, Name: pb.Status.JobName}, job); err != nil {
				if errors.IsNotFound(err) {
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

func (r *PackerBuilderReconciler) watchJobCompletion(ctx context.Context, namespace, jobName string, resultChan chan<- BuildResult, errChan chan<- error) {
	watcher, err := r.Clientset.BatchV1().Jobs(namespace).Watch(ctx, metav1.SingleObject(metav1.ObjectMeta{Name: jobName}))
	if err != nil {
		errChan <- fmt.Errorf("failed to create job watcher: %w", err)
		return
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				errChan <- fmt.Errorf("job watch channel closed unexpectedly")
				return
			}

			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				errChan <- fmt.Errorf("unexpected object type from watch event: %T", event.Object)
				continue
			}

			if job.Status.Succeeded > 0 {
				resultChan <- BuildResult{Completed: true, Succeeded: true, Reason: "Job completed successfully"}
				return
			}

			if job.Status.Failed > 0 {
				resultChan <- BuildResult{Completed: true, Failed: true, Reason: r.getJobFailureReason(job)}
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
func isEmpty(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}
