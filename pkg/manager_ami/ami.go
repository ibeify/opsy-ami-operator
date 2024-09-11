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
package manager_ami

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/go-logr/logr"
	c "github.com/ibeify/opsy-ami-operator/pkg/client"
	"github.com/ibeify/opsy-ami-operator/pkg/status"

	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
)

var (
	PackerBuilderCheck = AMITypeCheck{
		ImageActive:           false,
		ImageCheckPassDefault: false,
		ImageExist:            false,
		ImageExpired:          true,
		ImageNotFound:         true,
	}

	AMIRefreshCheck = AMITypeCheck{
		ImageActive:           true,
		ImageCheckPassDefault: false,
		ImageExist:            true,
		ImageExpired:          false,
		ImageNotFound:         false,
	}
)

type AMITypeCheck struct {
	ImageActive           bool
	ImageCheckPassDefault bool
	ImageExist            bool
	ImageExpired          bool
	ImageNotFound         bool
}

type ManagerAMI struct {
	ec2Client *ec2.Client
	log       logr.Logger
}

func New(ctx context.Context, region string, log logr.Logger) *ManagerAMI {
	cfg, err := c.ClientAWS(ctx, region)
	if err != nil {
		log.Error(err, "failed to create AWS client")
		return nil
	}
	return &ManagerAMI{

		ec2Client: ec2.NewFromConfig(cfg),
		log:       log,
	}
}
func calculateRemainingTime(expirationTime time.Time) string {
	now := time.Now()
	duration := expirationTime.Sub(now)
	if duration < 0 {
		return "Expired"
	}
	return fmt.Sprintf("%v", duration.Round(time.Second))
}

func (m *ManagerAMI) BaseAMIExists(ctx context.Context, fltrs []amiv1alpha1.AMIFilters) status.Result {
	return m.runWithTimeout(ctx, func(ctx context.Context) status.Result {
		if len(fltrs) == 0 {
			return status.Result{Success: false, Message: "No AMI filters provided; skipping base AMI check"}
		}

		var filters []types.Filter
		for _, filter := range fltrs {
			filters = append(filters, types.Filter{
				Name:   aws.String(filter.Name),
				Values: filter.Values,
			})
		}

		images, err := m.DescribeImagesWithFilters(ctx, filters)
		if err != nil {
			if strings.Contains(err.Error(), "no AMIs found matching the provided filters") {
				return status.Result{Success: true, Message: "No AMIs found matching the provided tags"}
			}
			return status.Result{Success: false, Error: err}
		}

		latestImage, _, err := m.FindLatestImage(images)
		if err != nil {
			return status.Result{Success: false, Error: err}
		}

		if latestImage == nil {
			return status.Result{
				Success: true,
				Message: "No AMIs found with a valid create-timestamp",
			}
		}

		return status.Result{
			Success: true,
			Message: fmt.Sprintf("Latest Base AMI found from filters: %s", *latestImage.ImageId),
			Name:    *latestImage.ImageId,
		}
	}, 60*time.Second)
}
func (m *ManagerAMI) AMIChecks(ctx context.Context, tags map[string]string, duration time.Duration, checkType AMITypeCheck) status.Result {
	return m.runWithTimeout(ctx, func(ctx context.Context) status.Result {
		filters := m.BuildFilters(tags)

		images, err := m.DescribeImagesWithFilters(ctx, filters)
		if err != nil {
			return status.Result{Success: false, Error: fmt.Errorf("failed to describe images: %w", err)}
		}

		if len(images) == 0 {
			return status.Result{Success: checkType.ImageNotFound, Message: "No AMIs found matching the provided tags"}
		}

		latestImage, createdTimestamp, err := m.FindLatestImage(images)
		if err != nil {
			return status.Result{Success: checkType.ImageNotFound, Message: "No AMIs found"}
		}

		if latestImage == nil {
			return status.Result{Success: checkType.ImageExist, Message: "No AMIs found with a valid create-timestamp"}
		}

		m.log.V(1).Info("latest AMI found", "image", *latestImage.ImageId, "timestamp", createdTimestamp)

		if m.IsImageExpired(createdTimestamp, duration) {
			return status.Result{
				Success: checkType.ImageExpired,
				Message: fmt.Sprintf("Latest Created AMI %s has expired", *latestImage.ImageId),
				Name:    *latestImage.ImageId,
			}
		}

		timeLeftBeforeExpirationDate := calculateRemainingTime(createdTimestamp.Add(duration))

		m.log.V(1).Info("time left before expiration", "image", *latestImage.ImageId, "timeLeftBeforeExpiration", timeLeftBeforeExpirationDate)

		if m.IsImageActive(latestImage) {
			return status.Result{
				Success: checkType.ImageActive,
				Message: fmt.Sprintf("Latest Created AMI %s is active", *latestImage.ImageId),
				Name:    *latestImage.ImageId,
			}
		}

		return status.Result{
			Success: true,
			Message: fmt.Sprintf("Valid AMI %s exist and has not expired,however it is not ACTIVE, created at %s", *latestImage.ImageId, createdTimestamp),
			Name:    *latestImage.ImageId,
		}
	}, 45*time.Second)
}

func (m *ManagerAMI) BuildFilters(tags map[string]string) []types.Filter {
	var filters []types.Filter
	for key, value := range tags {
		filters = append(filters, types.Filter{
			Name:   aws.String(fmt.Sprintf("tag:%s", key)),
			Values: []string{value},
		})
	}
	return filters
}

// describeImagesWithFilters describes images using the AWS EC2 API with the provided filters
func (m *ManagerAMI) DescribeImagesWithFilters(ctx context.Context, filters []types.Filter) ([]types.Image, error) {
	input := &ec2.DescribeImagesInput{
		Filters: filters,
	}
	result, err := m.ec2Client.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe images: %w", err)
	}
	return result.Images, nil
}

// findLatestImage finds the latest image from a list of images based on their timestamps
func (m *ManagerAMI) FindLatestImage(images []types.Image) (*types.Image, time.Time, error) {
	var latestImage *types.Image
	var createdTimestamp time.Time

	for i, image := range images {
		var imageTimestamp time.Time
		var err error

		// First, try to get the timestamp from the create-timestamp tag
		for _, tag := range image.Tags {
			if tag.Key != nil && *tag.Key == "create-timestamp" {
				imageTimestamp, err = time.Parse(time.RFC3339, *tag.Value)
				if err != nil {
					return nil, time.Time{}, fmt.Errorf("failed to parse create-timestamp for AMI %s: %v", *image.ImageId, err)
				}
				break
			}
		}

		// If create-timestamp tag is not found, use CreationDate
		if imageTimestamp.IsZero() && image.CreationDate != nil {
			imageTimestamp, err = time.Parse(time.RFC3339, *image.CreationDate)
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("failed to parse CreationDate for AMI %s: %v", *image.ImageId, err)
			}
		}

		// Update latest image if this is the first image or if it's newer
		if i == 0 || imageTimestamp.After(createdTimestamp) {
			latestImage = &images[i]
			createdTimestamp = imageTimestamp
		}
	}

	if latestImage == nil {
		return nil, time.Time{}, fmt.Errorf("no valid images found")
	}

	return latestImage, createdTimestamp, nil
}

func (m *ManagerAMI) IsImageActive(image *types.Image) bool {
	for _, tag := range image.Tags {
		if tag.Key != nil && tag.Value != nil && *tag.Key == "status" && *tag.Value == "active" {
			return true
		}
	}
	return false
}

func (m *ManagerAMI) IsImageExpired(creationTime time.Time, duration time.Duration) bool {
	return time.Now().UTC().After(creationTime.Add(duration))
}

func (m *ManagerAMI) runWithTimeout(ctx context.Context, op func(context.Context) status.Result, timeout time.Duration) status.Result {
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
