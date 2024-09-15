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

package notifier

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// SNSService implements the Amazon SNS notification service
type SNSService struct {
	client   *sns.Client
	topicARN string
}

// NewSNSService creates a new Amazon SNS service
func NewSNSService(ctx context.Context, region, topicARN string) (*SNSService, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	return &SNSService{
		client:   sns.NewFromConfig(cfg),
		topicARN: topicARN,
	}, nil
}

// Send implements the NotificationService interface for SNSService
func (s *SNSService) Send(ctx context.Context, subject, message string) error {
	input := &sns.PublishInput{
		Message:  aws.String(message),
		Subject:  aws.String(subject),
		TopicArn: aws.String(s.topicARN),
	}

	_, err := s.client.Publish(ctx, input)
	return err
}
