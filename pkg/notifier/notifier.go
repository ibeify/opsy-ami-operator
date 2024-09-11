package notifier

import (
	"context"
	"fmt"
	"sync"
)

// NotificationService defines the interface for all notification services
type NotificationService interface {
	Send(ctx context.Context, subject, message string) error
}

// MultiNotifier manages multiple notification services
type MultiNotifier struct {
	services []NotificationService
}

// NewMultiNotifier creates a new MultiNotifier
func NewMultiNotifier() *MultiNotifier {
	return &MultiNotifier{
		services: []NotificationService{},
	}
}

// AddService adds a new notification service to the MultiNotifier
func (mn *MultiNotifier) AddService(service NotificationService) {
	mn.services = append(mn.services, service)
}

// Send sends a notification to all added services concurrently
func (mn *MultiNotifier) Send(ctx context.Context, subject, message string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(mn.services))

	for _, service := range mn.services {
		wg.Add(1)
		go func(s NotificationService) {
			defer wg.Done()
			if err := s.Send(ctx, subject, message); err != nil {
				errChan <- err
			}
		}(service)
	}

	wg.Wait()
	close(errChan)

	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while sending notifications: %v", errors)
	}

	return nil
}
