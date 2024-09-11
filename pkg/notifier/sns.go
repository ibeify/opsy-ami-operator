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
