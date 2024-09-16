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
package manager_packer

import (
	"context"
	"fmt"
	"path/filepath"

	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	configurations "github.com/ibeify/opsy-ami-operator/pkg/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type command []string

func (r *ManagerPacker) GetCommand(cmd command) command {
	return cmd
}

func (r *ManagerPacker) Job(ctx context.Context, pb *amiv1alpha1.PackerBuilder) (*batchv1.Job, error) {

	var envVars []corev1.EnvVar
	var gitSyncSecretEnv []corev1.EnvVar

	pb.Spec.Builder.Debug = true // Always run in debug mode for now
	podReplacementPolicy := batchv1.Failed

	if pb.Spec.Builder.Debug {
		envVars = append(envVars, corev1.EnvVar{Name: "PACKER_LOG", Value: "1"}, corev1.EnvVar{Name: "AWS_SDK_GO_V2", Value: "debug"})
	}

	cmd := command{
		"sh",
		"-c",
	}

	labels := map[string]string{
		"brought-to-you-by": "opsy-the-ami-operator",
		"job-name":          pb.Status.JobName,
		"created-by":        pb.Name,
		"cluster":           pb.Spec.ClusterName,
		"job-id":            string(pb.Status.JobID),
	}

	initEnvVars := []corev1.EnvVar{
		{
			Name:  "GITSYNC_ROOT",
			Value: filepath.Join("/" + configurations.WorkingDirRoot),
		},
		{
			Name:  "GITSYNC_LINK",
			Value: filepath.Join("/"+configurations.WorkingDirRoot, configurations.WorkingDir),
		},
		{
			Name:  "GITSYNC_REPO",
			Value: pb.Spec.Builder.RepoURL,
		},
		{
			Name:  "GITSYNC_REF",
			Value: pb.Spec.Builder.Branch,
		},
		{
			Name:  "GITSYNC_ONE_TIME",
			Value: "true",
		},
		{
			Name:  "GITSYNC_WAIT",
			Value: "30",
		},
		{
			Name:  "GITSYNC_DEPTH",
			Value: "1",
		},
		{
			Name:  "GITSYNC_VERBOSE",
			Value: "2",
		},
	}

	if pb.Spec.GitSync.Secret != "" {
		gitSyncSecretEnv = []corev1.EnvVar{
			{
				Name: "GITSYNC_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pb.Spec.GitSync.Secret,
						},
						Key: "username",
					},
				},
			},
			{
				Name: "GITSYNC_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pb.Spec.GitSync.Secret,
						},
						Key: "token",
					},
				},
			},
		}
	}

	if pb.Spec.Builder.Secret != "" {

		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: pb.Spec.Builder.Secret, Namespace: pb.Namespace}, secret); err != nil {
			return nil, err
		}

		for key := range secret.Data {
			envVars = append(envVars, corev1.EnvVar{
				Name: key,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: pb.Spec.Builder.Secret,
						},
						Key: fmt.Sprintf("%v", key),
					},
				},
			})
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.Status.JobName,
			Namespace: pb.Namespace,
			Labels:    labels,
		},

		Spec: batchv1.JobSpec{
			PodReplacementPolicy:    &podReplacementPolicy,
			TTLSecondsAfterFinished: r.int32Ptr(900),
			Completions:             r.int32Ptr(1),
			BackoffLimit:            r.int32Ptr(0),

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Name:   pb.Status.JobName,
				},

				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:            pb.Spec.GitSync.Name,
							Image:           pb.Spec.GitSync.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env:             append(initEnvVars, gitSyncSecretEnv...),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configurations.WorkingDirRoot,
									MountPath: "/" + configurations.WorkingDirRoot,
								},
							},
						},
					},

					ServiceAccountName: func() string {
						if pb.Spec.Builder.JobServiceAccountName != "" {
							return pb.Spec.Builder.JobServiceAccountName
						}
						return ""
					}(),

					Containers: []corev1.Container{
						{
							Name:            "packer-runner",
							Env:             envVars,
							Image:           pb.Spec.Builder.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"/bin/sh", "-c", "kill -TERM 1"},
									},
								},
							},

							Command: r.GetCommand(cmd),

							Args: []string{
								fmt.Sprintf("echo 'Executing command:' && echo '%s' && %s",
									pb.Status.Command,
									pb.Status.Command,
								),
							},

							WorkingDir: filepath.Join("/"+configurations.WorkingDirRoot, configurations.WorkingDir),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      configurations.WorkingDirRoot,
									MountPath: "/" + configurations.WorkingDirRoot,
									ReadOnly:  false,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: configurations.WorkingDirRoot,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return job, nil
}

func (m *ManagerPacker) int32Ptr(i int32) *int32 {
	return &i
}

func (m *ManagerPacker) UpdateFailedJobCount(ctx context.Context, packbuilder *amiv1alpha1.PackerBuilder) error {
	jobList := &batchv1.JobList{}
	if err := m.Client.List(ctx, jobList, client.InNamespace(packbuilder.Namespace), client.MatchingLabels{
		"brought-to-you-by": "opsy-the-ami-operator",
		"packer-builder":    packbuilder.Name,
	}); err != nil {
		return err
	}

	failedJobCount := int32(0)
	for _, job := range jobList.Items {
		if job.Status.Failed > 0 {
			failedJobCount++
		}
	}

	if packbuilder.Status.FailedJobCount != &failedJobCount {
		packbuilder.Status.FailedJobCount = &failedJobCount
		if err := m.Status.Update(ctx, packbuilder); err != nil {
			return err
		}
	}

	return nil
}
