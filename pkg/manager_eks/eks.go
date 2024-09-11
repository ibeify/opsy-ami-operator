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
package manager_eks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/go-logr/logr"
	c "github.com/ibeify/opsy-ami-operator/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type UpdateConfig struct {
	Enable                   bool
	Force                    bool
	WaitForCompletion        bool
	MaxUnavailable           *int32
	MaxUnavailablePercentage *int32
}

type ManagerRefresh struct {
	client.Client
	clusterName string
	log         logr.Logger
	eksClient   *eks.Client
	ec2Client   *ec2.Client
}

func New(ctx context.Context, client client.Client, region, clusterName string, log logr.Logger) *ManagerRefresh {

	cfg, err := c.ClientAWS(ctx, region)
	if err != nil {
		log.Error(err, "failed to create AWS client")
		return nil
	}

	return &ManagerRefresh{
		Client:      client,
		clusterName: clusterName,
		eksClient:   eks.NewFromConfig(cfg),
		ec2Client:   ec2.NewFromConfig(cfg),
		log:         log,
	}
}

func (r *ManagerRefresh) UpdateNodeGroupAMI(ctx context.Context, nodeGroupName, newAMIID string, updateConfig UpdateConfig) error {
	log := r.log.WithValues("cluster", r.clusterName, "nodeGroup", nodeGroupName, "newAMI", newAMIID)

	if err := ctx.Err(); err != nil {
		log.Error(err, "Context cancelled before starting the update")
		return err
	}

	if err := r.updateNodeGroupConfig(ctx, nodeGroupName, &updateConfig); err != nil {
		return err
	}

	describeInput := &eks.DescribeNodegroupInput{
		ClusterName:   &r.clusterName,
		NodegroupName: &nodeGroupName,
	}
	nodeGroup, err := r.eksClient.DescribeNodegroup(ctx, describeInput)
	if err != nil {
		log.Error(err, "Failed to describe node group")
		return err
	}

	if err := ctx.Err(); err != nil {
		log.Error(err, "Context cancelled after describing node group")
		return err
	}

	ltName := aws.ToString(nodeGroup.Nodegroup.LaunchTemplate.Name)
	ltVersion := aws.ToString(nodeGroup.Nodegroup.LaunchTemplate.Version)

	descLTInput := &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateName: aws.String(ltName),
		Versions:           []string{ltVersion},
	}
	ltVersions, err := r.ec2Client.DescribeLaunchTemplateVersions(ctx, descLTInput)
	if err != nil {
		return fmt.Errorf("failed to describe launch template version: %w", err)
	}

	if len(ltVersions.LaunchTemplateVersions) == 0 {
		return fmt.Errorf("no launch template versions found")
	}

	currentVersion := ltVersions.LaunchTemplateVersions[0].LaunchTemplateData

	if !updateConfig.Force && strings.TrimPrefix(*currentVersion.ImageId, "$Latest-") == newAMIID {
		log.Info("Node group is already using the specified AMI ID. No update needed.", "currentAMI", *currentVersion)
		return nil
	}

	createLTVersionInput := &ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateName: aws.String(ltName),
		SourceVersion:      aws.String(ltVersion),
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
			ImageId: aws.String(newAMIID),
		},
	}

	newLTVersion, err := r.ec2Client.CreateLaunchTemplateVersion(ctx, createLTVersionInput)
	if err != nil {
		return fmt.Errorf("failed to create new launch template version: %w", err)
	}

	updateInput := &eks.UpdateNodegroupVersionInput{
		ClusterName:   &r.clusterName,
		NodegroupName: &nodeGroupName,
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
			Version: aws.String(fmt.Sprintf("%d", aws.ToInt64(newLTVersion.LaunchTemplateVersion.VersionNumber))),
		},
	}

	if err := ctx.Err(); err != nil {
		log.Error(err, "Context cancelled before initiating the update")
		return err
	}

	_, err = r.eksClient.UpdateNodegroupVersion(ctx, updateInput)
	if err != nil {
		log.Error(err, "Failed to update node group")
		return err
	}

	log.Info("Successfully initiated update for node group")

	if updateConfig.WaitForCompletion {
		log.Info("Waiting for node group update to complete")

		if err := ctx.Err(); err != nil {
			log.Error(err, "Context cancelled before initiating the update")
			return err
		}

		return r.waitForUpdateCompletion(ctx, nodeGroupName, log)
	}

	log.Info("Not waiting for update completion as WaitForCompletion is set to false")
	return nil
}

func (r *ManagerRefresh) updateNodeGroupConfig(ctx context.Context, nodeGroupName string, updateConfig *UpdateConfig) error {
	log := r.log.WithValues("cluster", r.clusterName, "nodeGroup", nodeGroupName)

	if updateConfig.Enable {
		if updateConfig.MaxUnavailable == nil && updateConfig.MaxUnavailablePercentage == nil {
			defaultMaxUnavailable := int32(1)
			updateConfig.MaxUnavailable = &defaultMaxUnavailable
			log.Info("Using default MaxUnavailable value", "maxUnavailable", defaultMaxUnavailable)
		}

		updateConfigInput := &eks.UpdateNodegroupConfigInput{
			ClusterName:   &r.clusterName,
			NodegroupName: &nodeGroupName,
			UpdateConfig: &ekstypes.NodegroupUpdateConfig{
				MaxUnavailable: updateConfig.MaxUnavailable,
			},
		}

		_, err := r.eksClient.UpdateNodegroupConfig(ctx, updateConfigInput)
		if err != nil {
			log.Error(err, "Failed to update node group configuration")
			return err
		}

		log.Info("Successfully updated node group configuration",
			"maxUnavailable", updateConfig.MaxUnavailable,
		)
	} else {
		log.Info("Skipping update of MaxUnavailable/MaxUnavailablePercentage as Enable is set to false")
	}

	return nil
}

func (r *ManagerRefresh) waitForUpdateCompletion(ctx context.Context, nodeGroupName string, log logr.Logger) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	type statusResult struct {
		Status ekstypes.NodegroupStatus
		Error  error
	}

	// Use a buffered channel for status updates
	statusChan := make(chan statusResult, 1)

	for {
		select {
		case <-ctx.Done():
			log.Info("Context cancelled while waiting for update completion")
			return ctx.Err()
		case <-ticker.C:
			// Use a goroutine to check status
			go func() {
				status, err := r.getNodeGroupStatus(ctx, nodeGroupName)
				statusChan <- statusResult{Status: status, Error: err}
			}()

		case result := <-statusChan:
			if result.Error != nil {
				log.Error(result.Error, "Failed to get node group status")
				continue
			}

			log.Info("Current node group status", "status", result.Status)

			switch result.Status {
			case ekstypes.NodegroupStatusActive:
				log.Info("Node group update completed successfully")
				return nil
			case ekstypes.NodegroupStatusDegraded:
				return r.handleUpdateFailure(ctx, nodeGroupName, log)
			case ekstypes.NodegroupStatusCreating, ekstypes.NodegroupStatusUpdating, ekstypes.NodegroupStatusDeleting:
				// Continue waiting
			default:
				log.Info("Unexpected node group status", "status", result.Status)
			}
		}
	}
}

func (r *ManagerRefresh) getNodeGroupStatus(ctx context.Context, nodeGroupName string) (ekstypes.NodegroupStatus, error) {
	describeInput := &eks.DescribeNodegroupInput{
		ClusterName:   &r.clusterName,
		NodegroupName: &nodeGroupName,
	}
	describeOutput, err := r.eksClient.DescribeNodegroup(ctx, describeInput)
	if err != nil {
		return "", err // Return empty string as status in case of error
	}
	return describeOutput.Nodegroup.Status, nil
}

func (r *ManagerRefresh) handleUpdateFailure(ctx context.Context, nodeGroupName string, log logr.Logger) error {
	describeInput := &eks.DescribeNodegroupInput{
		ClusterName:   &r.clusterName,
		NodegroupName: &nodeGroupName,
	}
	describeOutput, err := r.eksClient.DescribeNodegroup(ctx, describeInput)
	if err != nil {
		log.Error(err, "Failed to describe node group after update failure")
		return err
	}

	log.Error(nil, "Node group update failed or degraded", "status", describeOutput.Nodegroup.Status)

	if describeOutput.Nodegroup.Health != nil {
		for _, issue := range describeOutput.Nodegroup.Health.Issues {
			log.Error(nil, "Health issue detected",
				"code", issue.Code,
				"message", issue.Message,
				"resourceIds", issue.ResourceIds)
		}
	}

	return fmt.Errorf("node group update failed or degraded with status: %s", describeOutput.Nodegroup.Status)
}

// ListNodeGroups lists all node groups in a given EKS cluster
func (r *ManagerRefresh) ListNodeGroups(ctx context.Context, clusterName string) (*eks.ListNodegroupsOutput, error) {

	nodeGroups, err := r.eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: aws.String(clusterName),
	})

	if err != nil {
		var notFound *ekstypes.ResourceNotFoundException
		if ok := errors.As(err, &notFound); ok {
			return nil, fmt.Errorf("cluster %s not found", clusterName)
		}
		return nil, fmt.Errorf("failed to list node groups: %v", err)
	}

	return nodeGroups, nil
}

// trimNodeGroups removes the specified node groups from the list of node groups
func (r *ManagerRefresh) TrimNodeGroups(nodeGroups []string, excludeNodeGroupNames []string) []string {
	excludeMap := make(map[string]struct{}, len(excludeNodeGroupNames))
	for _, name := range excludeNodeGroupNames {
		excludeMap[name] = struct{}{}
	}

	trimmedNodeGroups := make([]string, 0, len(nodeGroups)-len(excludeNodeGroupNames))
	for _, nodeGroupName := range nodeGroups {
		if _, found := excludeMap[nodeGroupName]; !found {
			trimmedNodeGroups = append(trimmedNodeGroups, nodeGroupName)
		}
	}
	return trimmedNodeGroups
}
