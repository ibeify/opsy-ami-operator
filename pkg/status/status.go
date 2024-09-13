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

package status

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// operation represents a function that performs some action and returns a Result
type Operation func(context.Context) Result

type Result struct {
	Success bool
	Message string
	Name    string
	Error   error
}

// ObjectWithStatus is an interface for objects that have a status that can be updated
type ObjectWithStatus interface {
	client.Object
	runtime.Object
}

// StatusUpdater is a generic struct for updating status
type Status struct {
	Client client.Client
}

func New(client client.Client) *Status {
	return &Status{
		Client: client,
	}
}

func (s *Status) Update(ctx context.Context, obj client.Object) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get the latest version of the object
		latestObj := obj.DeepCopyObject().(client.Object)
		if err := s.Client.Get(ctx, client.ObjectKeyFromObject(obj), latestObj); err != nil {
			return fmt.Errorf("failed to get latest version of object: %w", err)
		}

		// Copy the status from obj to latestObj
		statusField := reflect.ValueOf(latestObj).Elem().FieldByName("Status")
		if statusField.IsValid() && statusField.CanSet() {
			statusField.Set(reflect.ValueOf(obj).Elem().FieldByName("Status"))
		}

		// Update the status
		if err := s.Client.Status().Update(ctx, latestObj); err != nil {
			if errors.IsConflict(err) {
				return err
			}

			return fmt.Errorf("failed to update object status: %w", err)
		}
		return nil
	})
}

// func (s *Status) Update(ctx context.Context, obj client.Object) error {
// 	return wait.ExponentialBackoff(wait.Backoff{
// 		Steps:    5,
// 		Duration: 100 * time.Millisecond,
// 		Factor:   2.0,
// 		Jitter:   0.1,
// 	}, func() (bool, error) {

// 		latestObj := obj.DeepCopyObject().(client.Object)
// 		if err := s.Client.Get(ctx, client.ObjectKeyFromObject(obj), latestObj); err != nil {
// 			return false, fmt.Errorf("failed to get latest version of object: %w", err)
// 		}

// 		// Copy the status from obj to latestObj
// 		statusField := reflect.ValueOf(latestObj).Elem().FieldByName("Status")
// 		if statusField.IsValid() && statusField.CanSet() {
// 			statusField.Set(reflect.ValueOf(obj).Elem().FieldByName("Status"))
// 		}

// 		// Update the status
// 		if err := s.Client.Status().Update(ctx, latestObj); err != nil {
// 			if errors.IsConflict(err) {
// 				return false, err
// 			}

// 			return false, fmt.Errorf("failed to update object status: %w", err)
// 		}

// 		return true, nil
// 	})
// }
