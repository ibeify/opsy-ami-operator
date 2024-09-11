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

package events

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
)

type EventManager struct {
	recorder record.EventRecorder
	mu       sync.Mutex
}

func New(recorder record.EventRecorder) *EventManager {
	return &EventManager{
		recorder: recorder,
	}
}

func (em *EventManager) RecordEvent(object runtime.Object, eventtype, reason, message string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.recorder.Event(object, eventtype, reason, message)
}

func (em *EventManager) RecordNormalEvent(object runtime.Object, reason, message string) {
	em.RecordEvent(object, corev1.EventTypeNormal, reason, message)
}

func (em *EventManager) RecordWarningEvent(object runtime.Object, reason, message string) {
	em.RecordEvent(object, corev1.EventTypeWarning, reason, message)
}
