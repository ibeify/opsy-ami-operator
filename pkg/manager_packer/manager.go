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
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
	c "github.com/ibeify/opsy-ami-operator/pkg/client"
	configurations "github.com/ibeify/opsy-ami-operator/pkg/config"
	manager_ami "github.com/ibeify/opsy-ami-operator/pkg/manager_ami"
	"github.com/ibeify/opsy-ami-operator/pkg/status"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FilterOptions struct {
	IncludeTimestamp   bool
	IncludeIdentifiers bool
	IncludeBaseAMI     bool
}

type RegExsy struct {
	Name        string
	Pattern     *regexp.Regexp
	Found       *bool
	StatusField func(match string)
}

type ManagerPacker struct {
	client.Client
	ClientAWS     aws.Config
	Clientset     *kubernetes.Clientset
	Config        *configurations.Config
	Log           logr.Logger
	PackerBuilder *amiv1alpha1.PackerBuilder
	Scheme        *runtime.Scheme
	Status        *status.Status
}

func New(ctx context.Context, client client.Client, log logr.Logger, scheme *runtime.Scheme, clientset *kubernetes.Clientset, packbuilder *amiv1alpha1.PackerBuilder) *ManagerPacker {
	cfg, err := c.ClientAWS(ctx, packbuilder.Spec.Region)
	if err != nil {
		return nil
	}
	return &ManagerPacker{
		Client:        client,
		ClientAWS:     cfg,
		Log:           log,
		Scheme:        scheme,
		Config:        configurations.NewConfig(),
		Clientset:     clientset,
		PackerBuilder: packbuilder,
		Status:        status.New(client),
	}
}

func (m *ManagerPacker) ActiveBuildCheck(ctx context.Context, namespace, name string) status.Result {

	return m.runWithTimeout(ctx, func(ctx context.Context) status.Result {
		if m == nil || m.Client == nil {
			return status.Result{Success: false, Error: fmt.Errorf("ManagerPacker or its Client is nil")}
		}

		latestPB := &amiv1alpha1.PackerBuilder{}
		if err := m.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, latestPB); err != nil {
			if k8serrors.IsNotFound(err) {
				return status.Result{Success: true, Message: "PackerBuilder has been deleted"}
			}
			return status.Result{Success: false, Error: fmt.Errorf("failed to get latest version of PackerBuilder: %w", err)}
		}

		m.PackerBuilder = latestPB

		builds := &batchv1.JobList{}
		if err := m.List(ctx, builds, client.InNamespace(namespace), client.MatchingLabels{"created-by": name}); err != nil {
			return status.Result{Success: false, Error: fmt.Errorf("unable to list jobs: %w", err)}
		}

		activeBuilds, activeJobName := m.getActiveBuilds(builds)

		if err := m.updatePackerBuilderStatus(ctx, activeBuilds); err != nil {
			m.Log.Error(err, "Failed to update PackerBuilder status", "name", name)
		}

		if len(activeBuilds) == 0 {
			return status.Result{Success: true, Message: "No active build found"}
		} else {
			return status.Result{Success: false, Message: "An active build exists, skipping job creation", Name: activeJobName}
		}
	}, 30*time.Second) // Adjust timeout as needed
}

func (m *ManagerPacker) DeleteKeyPair(ctx context.Context, keyPairName string) error {
	ec2Client := ec2.NewFromConfig(m.ClientAWS)
	_, err := ec2Client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{
		KeyNames: []string{keyPairName},
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "InvalidKeyPair.NotFound":
				// Key pair doesn't exist, so we can consider this a success
				return nil
			}
		}
		// If it's a different error, return it
		return err
	}

	// If we've reached here, the key pair exists, so delete it
	_, err = ec2Client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyName: aws.String(keyPairName),
	})
	return err
}

func (m *ManagerPacker) DeleteSecurityGroup(ctx context.Context, securityGroupID string) error {
	ec2Client := ec2.NewFromConfig(m.ClientAWS)
	_, err := ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{securityGroupID},
	})

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "InvalidGroup.NotFound":
				// Security group doesn't exist, so we can consider this a success
				return nil
			}
		}
		// If it's a different error, return it
		return err
	}

	// If we've reached here, the security group exists, so delete it
	_, err = ec2Client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(securityGroupID),
	})
	return err
}

func (m *ManagerPacker) LatestAMIDoesNotExistsOrExpired(ctx context.Context, duration time.Duration, packerBuilder *amiv1alpha1.PackerBuilder) status.Result {
	adoptResult := m.adoptionProcessor(ctx, packerBuilder)

	if !adoptResult.Success {
		return adoptResult
	}

	strict := false // Hardcoded for now
	options := FilterOptions{
		IncludeTimestamp:   false,
		IncludeIdentifiers: strict,
		IncludeBaseAMI:     strict,
	}

	ami := manager_ami.New(ctx, m.ClientAWS.Region, m.Log)
	tags := m.buildTagsAndFilters(packerBuilder, options)

	return ami.AMIChecks(ctx, tags, duration, manager_ami.PackerBuilderCheck)
}

func (m *ManagerPacker) adoptExistingImage(ctx context.Context, packerBuilder *amiv1alpha1.PackerBuilder) (*types.Image, error) {
	options := FilterOptions{
		IncludeTimestamp:   false,
		IncludeIdentifiers: false,
		IncludeBaseAMI:     false,
	}

	ami := manager_ami.New(ctx, m.ClientAWS.Region, m.Log)
	tags := m.buildTagsAndFilters(packerBuilder, options)

	filters := ami.BuildFilters(tags)
	images, err := ami.DescribeImagesWithFilters(ctx, filters)
	if err != nil {
		return nil, err
	}

	latestImage, _, err := ami.FindLatestImage(images)
	if err != nil || latestImage == nil {

		return nil, err
	}

	if !ami.IsImageActive(latestImage) {
		return nil, fmt.Errorf("latest AMI %s is not active", *latestImage.ImageId)
	}

	packerBuilder.Status.LastRunBuiltImageID = *latestImage.ImageId
	packerBuilder.Status.LastRunBaseImageID = m.getBaseAMIID(*latestImage)
	packerBuilder.Status.BuildID = m.generateNewBuildID()

	if err := m.Status.Update(ctx, packerBuilder); err != nil {
		return nil, fmt.Errorf("failed to update PackerBuilder status: %v", err)
	}

	if err := m.tagAMI(ctx, *latestImage.ImageId, packerBuilder); err != nil {
		return nil, fmt.Errorf("failed to tag adopted AMI: %v", err)
	}

	deprecateOptions := FilterOptions{
		IncludeTimestamp:   false,
		IncludeIdentifiers: false,
		IncludeBaseAMI:     true,
	}

	if err := m.deprecateOldAMIs(ctx, packerBuilder, *latestImage.ImageId, deprecateOptions); err != nil {
		return nil, fmt.Errorf("failed to deprecate old AMIs: %v", err)
	}

	return latestImage, nil
}

func (m *ManagerPacker) adoptionProcessor(ctx context.Context, packerBuilder *amiv1alpha1.PackerBuilder) status.Result {
	return m.runWithTimeout(ctx, func(ctx context.Context) status.Result {
		if packerBuilder.Status.LastRunBuiltImageID == "" {
			m.Log.Info("Attempting to adopt existing image")
			adoptedImage, err := m.adoptExistingImage(ctx, packerBuilder)
			if err != nil {
				if strings.Contains(err.Error(), "no AMIs found matching the provided filters") || strings.Contains(err.Error(), "no valid images found") {
					return status.Result{Success: true, Message: "No AMIs found matching the provided tags"}
				}
				return status.Result{Success: false, Error: err}
			}
			if adoptedImage != nil {
				m.Log.Info("Adopted existing image", "image", *adoptedImage.ImageId)
				return status.Result{Success: false, Message: fmt.Sprintf("Adopted existing AMI %s", *adoptedImage.ImageId)}
			}
		}
		return status.Result{Success: true, Message: "No adoption needed or adoption successful"}
	}, 45*time.Second)
}

func (m *ManagerPacker) buildTagsAndFilters(packerBuilder *amiv1alpha1.PackerBuilder, options FilterOptions) map[string]string {
	tags := map[string]string{
		"brought-to-you-by":  "opsy-the-ami-operator",
		"cluster-name":       packerBuilder.Spec.ClusterName,
		"created-by":         packerBuilder.Name,
		"packer-repo":        packerBuilder.Spec.Builder.RepoURL,
		"packer-repo-branch": packerBuilder.Spec.Builder.Branch,
	}

	if options.IncludeBaseAMI {
		tags["base-ami"] = packerBuilder.Status.LastRunBaseImageID
	}

	if options.IncludeIdentifiers {
		tags["job-id"] = packerBuilder.Status.JobID
		tags["build-id"] = packerBuilder.Status.BuildID
	}

	if options.IncludeTimestamp {
		tags["create-timestamp"] = time.Now().UTC().Format(time.RFC3339)
		tags["status"] = "active"
	}

	return tags
}

func (m *ManagerPacker) deprecateAMI(ctx context.Context, ec2Client *ec2.Client, amiID string) error {
	_, err := ec2Client.ModifyImageAttribute(ctx, &ec2.ModifyImageAttributeInput{
		ImageId:       aws.String(amiID),
		OperationType: types.OperationTypeAdd,
		LaunchPermission: &types.LaunchPermissionModifications{
			Remove: []types.LaunchPermission{{Group: types.PermissionGroupAll}},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to deprecate AMI %s: %v", amiID, err)
	}
	_, err = ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{amiID},
		Tags: []types.Tag{{
			Key:   aws.String("status"),
			Value: aws.String("deprecated"),
		}},
	})
	if err != nil {
		return fmt.Errorf("failed to tag AMI %s as deprecated: %v", amiID, err)
	}
	return nil
}

func (m *ManagerPacker) deprecateOldAMIs(ctx context.Context, packerBuilder *amiv1alpha1.PackerBuilder, excludeAMI string, options FilterOptions) error {

	ec2Client := ec2.NewFromConfig(m.ClientAWS)
	ami := manager_ami.New(ctx, m.ClientAWS.Region, m.Log)
	tags := m.buildTagsAndFilters(packerBuilder, options)

	filters := ami.BuildFilters(tags)

	images, err := ami.DescribeImagesWithFilters(ctx, filters)
	if err != nil {
		return err
	}

	for _, image := range images {
		if *image.ImageId == excludeAMI {
			continue
		}
		if err := m.deprecateAMI(ctx, ec2Client, *image.ImageId); err != nil {
			return err
		}
	}

	return nil
}

func (m *ManagerPacker) getActiveBuilds(builds *batchv1.JobList) ([]*batchv1.Job, string) {
	var activeBuilds []*batchv1.Job
	var activeJobName string

	for i, build := range builds.Items {
		isFinished, _ := m.isBuildFinished(&build)
		if !isFinished {
			activeBuilds = append(activeBuilds, &builds.Items[i])
			activeJobName = build.Name
		}
	}

	return activeBuilds, activeJobName
}

func (m *ManagerPacker) generateNewBuildID() string {
	return uuid.New().String()
}

func (m *ManagerPacker) getBaseAMIID(image types.Image) string {
	for _, tag := range image.Tags {
		if *tag.Key == "base-ami" {
			return *tag.Value
		}
	}
	return ""
}

func (m *ManagerPacker) handleAMIUpdates(ctx context.Context, packerBuilder *amiv1alpha1.PackerBuilder) status.Result {
	return m.runWithTimeout(ctx, func(ctx context.Context) status.Result {
		amiID := packerBuilder.Status.LastRunBuiltImageID

		if err := m.tagAMI(ctx, amiID, packerBuilder); err != nil {
			return status.Result{Success: false, Error: err}
		}

		options := FilterOptions{
			IncludeTimestamp:   false,
			IncludeIdentifiers: false,
			IncludeBaseAMI:     true,
		}
		if err := m.deprecateOldAMIs(ctx, packerBuilder, amiID, options); err != nil {
			return status.Result{Success: false, Error: err}
		}
		return status.Result{Success: true, Message: "Successfully tagged the new AMI and deprecated old AMIs"}
	}, 120*time.Second)
}

func (m *ManagerPacker) isBuildFinished(job *batchv1.Job) (bool, batchv1.JobConditionType) {
	for _, c := range job.Status.Conditions {
		if (c.Type == batchv1.JobComplete || c.Type == batchv1.JobFailed) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}

func (m *ManagerPacker) runWithTimeout(ctx context.Context, op status.Operation, timeout time.Duration) status.Result {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultCh := make(chan status.Result, 1)
	go func() {
		resultCh <- op(ctx)
	}()

	select {
	case <-ctx.Done():
		return status.Result{Success: false, Error: ctx.Err()}
	case result := <-resultCh:
		return result
	}
}

func (m *ManagerPacker) tagAMI(ctx context.Context, amiID string, packerBuilder *amiv1alpha1.PackerBuilder) error {

	tagOptions := FilterOptions{
		IncludeTimestamp:   true,
		IncludeIdentifiers: true,
		IncludeBaseAMI:     true,
	}

	ec2Client := ec2.NewFromConfig(m.ClientAWS)
	tags := m.buildTagsAndFilters(packerBuilder, tagOptions)
	var ec2Tags []types.Tag
	for key, value := range tags {
		ec2Tags = append(ec2Tags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	_, err := ec2Client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{amiID},
		Tags:      ec2Tags,
	})
	if err != nil {
		if awsErr, ok := err.(*smithy.OperationError); ok && strings.Contains(awsErr.Error(), "InvalidID") {
			return fmt.Errorf("failed to tag AMI %s: Invalid AMI ID: %v", amiID, err)
		}
		return fmt.Errorf("failed to tag AMI %s: %v", amiID, err)
	}
	return nil
}

func (m *ManagerPacker) updatePackerBuilderStatus(ctx context.Context, activeBuilds []*batchv1.Job) error {
	if m.PackerBuilder == nil {
		return fmt.Errorf("PackerBuilder is nil")
	}

	m.PackerBuilder.Status.Active = []corev1.ObjectReference{}
	for _, activeJob := range activeBuilds {
		jobRef, err := ref.GetReference(m.Scheme, activeJob)
		if err != nil {
			m.Log.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		m.PackerBuilder.Status.Active = append(m.PackerBuilder.Status.Active, *jobRef)
	}

	if err := m.Status.Update(ctx, m.PackerBuilder); err != nil {
		return fmt.Errorf("failed to update PackerBuilder status: %w", err)
	}

	return nil
}
