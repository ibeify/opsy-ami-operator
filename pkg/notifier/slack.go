package notifier

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
)

// SlackService implements the Slack notification service
type SlackService struct {
	client  *slack.Client
	channel string
}

// BuildInfo holds additional information about the build
// BuildInfo holds additional information about the build
type Message struct {
	Name           string
	LatestBaseAMI  string
	LatestBuiltAMI string
	Message        string
	Status         string // New field for build status
}

// NewSlackService creates a new Slack service
func NewSlackService(token, channel string) *SlackService {
	return &SlackService{
		client:  slack.New(token),
		channel: channel,
	}
}

// Send implements the NotificationService interface for SlackService
func (s *SlackService) Send(ctx context.Context, subject string, m Message) error {
	// Define color based on status
	var color string
	switch m.Status {
	case "Success":
		color = "#36a64f" // Green
	case "Failure":
		color = "#ff0000" // Red
	default:
		color = "#f2c744" // Yellow for in-progress or unknown status
	}

	// Create a message with blocks for better formatting
	blocks := []slack.Block{
		slack.NewHeaderBlock(
			slack.NewTextBlockObject("plain_text", subject, false, false),
		),
		slack.NewContextBlock("",
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf(":gear: *Builder:* %s", m.Name), false, false),
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Status:* %s", m.Status), false, false),
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Base AMI:*\n%s", m.LatestBaseAMI), false, false),
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Built AMI:*\n%s", m.LatestBuiltAMI), false, false),
			},
			nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Details:*\n%s", m.Message), false, false),
			nil,
			nil,
		),
	}

	// Post the message to Slack
	_, _, err := s.client.PostMessageContext(ctx,
		s.channel,
		slack.MsgOptionBlocks(blocks...),
		slack.MsgOptionAttachments(slack.Attachment{Color: color}),
		slack.MsgOptionText(subject, false), // Fallback text
	)

	return err
}
