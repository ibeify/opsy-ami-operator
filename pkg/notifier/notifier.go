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
