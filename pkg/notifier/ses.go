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
