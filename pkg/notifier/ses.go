package notifier

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESService implements the Amazon SES notification service
type SESService struct {
	client *ses.Client
	sender string
	to     []string
}

// NewSESService creates a new Amazon SES service
func NewSESService(ctx context.Context, cfg aws.Config, sender string, to []string) *SESService {
	return &SESService{
		client: ses.NewFromConfig(cfg),
		sender: sender,
		to:     to,
	}
}

// Send implements the NotificationService interface for SESService
func (s *SESService) Send(ctx context.Context, subject, message string) error {
	input := &ses.SendEmailInput{
		Destination: &types.Destination{
			ToAddresses: s.to,
		},
		Message: &types.Message{
			Body: &types.Body{
				Text: &types.Content{
					Data:    aws.String(message),
					Charset: aws.String("UTF-8"),
				},
			},
			Subject: &types.Content{
				Data:    aws.String(subject),
				Charset: aws.String("UTF-8"),
			},
		},
		Source: aws.String(s.sender),
	}

	_, err := s.client.SendEmail(ctx, input)
	return err
}
