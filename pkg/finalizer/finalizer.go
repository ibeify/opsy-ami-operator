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

package finalizer

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func HandleFinalizer(ctx context.Context, obj client.Object, r client.Client, log logr.Logger) error {
	if obj.GetDeletionTimestamp().IsZero() {
		if !controllerutil.ContainsFinalizer(obj, "ami.opsy.dev/finalizer") {
			controllerutil.AddFinalizer(obj, "ami.opsy.dev/finalizer")
			log.Info(fmt.Sprintf("Add Finalizer %s", "ami.opsy.dev/finalizer"))

			err := r.Update(ctx, obj)
			if err != nil {
				log.Error(err, "Failed to update object")
				return err
			}

		}
	} else {
		if controllerutil.ContainsFinalizer(obj, "ami.opsy.dev/finalizer") {
			if err := deleteAssociatedJobs(ctx, obj, r, log); err != nil {
				log.Error(err, "Failed to delete associated jobs")
				return err
			}
			controllerutil.RemoveFinalizer(obj, "ami.opsy.dev/finalizer")
			log.Info(fmt.Sprintf("Remove Finalizer %s", "ami.opsy.dev/finalizer"))

			err := r.Update(ctx, obj)
			if err != nil {
				log.Error(err, "Failed to update object")
				return err
			}

		}
	}
	return nil
}

func deleteAssociatedJobs(ctx context.Context, obj client.Object, r client.Client, log logr.Logger) error {

	jobList := &batchv1.JobList{}
	if err := r.List(ctx, jobList,
		client.InNamespace(obj.GetNamespace()),
		client.MatchingLabels{"packer-builder": obj.GetName()},
	); err != nil {
		return fmt.Errorf("failed to list associated jobs: %w", err)
	}

	for i := range jobList.Items {
		job := &jobList.Items[i]
		if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			if !k8serrors.IsNotFound(err) {
				log.Error(err, "Failed to delete job", "job", job.Name)
				return fmt.Errorf("failed to delete job %s: %w", job.Name, err)
			}
		}
		log.Info("Deleted associated job", "job", job.Name)
	}

	return nil
}
