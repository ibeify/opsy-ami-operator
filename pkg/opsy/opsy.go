package opsy

import (
	"fmt"
	"sync"

	amiv1alpha1 "github.com/ibeify/opsy-ami-operator/api/ami/v1alpha1"
)

type OpsyRunner struct {
	states      map[string]amiv1alpha1.PackerBuilderState
	transitions map[amiv1alpha1.PackerBuilderState][]amiv1alpha1.PackerBuilderState
	mu          sync.RWMutex
}

func New() *OpsyRunner {
	return &OpsyRunner{
		states: make(map[string]amiv1alpha1.PackerBuilderState),
		transitions: map[amiv1alpha1.PackerBuilderState][]amiv1alpha1.PackerBuilderState{
			amiv1alpha1.StateInitial:      {amiv1alpha1.StateInitial, amiv1alpha1.StateFlightChecks, amiv1alpha1.StateError, amiv1alpha1.StateJobRunning, amiv1alpha1.StateJobCompleted, amiv1alpha1.StateJobCreation},
			amiv1alpha1.StateFlightChecks: {amiv1alpha1.StateInitial, amiv1alpha1.StateJobCreation, amiv1alpha1.StateJobRunning, amiv1alpha1.StateError},
			amiv1alpha1.StateJobCreation:  {amiv1alpha1.StateInitial, amiv1alpha1.StateJobRunning, amiv1alpha1.StateError},
			amiv1alpha1.StateJobRunning:   {amiv1alpha1.StateInitial, amiv1alpha1.StateJobCompleted, amiv1alpha1.StateJobFailed, amiv1alpha1.StateError},
			amiv1alpha1.StateJobCompleted: {amiv1alpha1.StateInitial, amiv1alpha1.StateAMICreated, amiv1alpha1.StateError},
			amiv1alpha1.StateJobFailed:    {amiv1alpha1.StateInitial, amiv1alpha1.StateError},
			amiv1alpha1.StateAMICreated:   {amiv1alpha1.StateInitial},
			amiv1alpha1.StateError:        {amiv1alpha1.StateInitial, amiv1alpha1.StateError},
		},
	}
}

func (sm *OpsyRunner) SetState(id string, state amiv1alpha1.PackerBuilderState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	currentState, exists := sm.states[id]
	if !exists {
		currentState = amiv1alpha1.StateInitial
	}

	if sm.isValidTransition(currentState, state) {
		sm.states[id] = state
		return nil
	}
	return fmt.Errorf("invalid state transition from %s to %s for id %s", currentState, state, id)
}

func (sm *OpsyRunner) DefaultState(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[id] = amiv1alpha1.StateInitial
}

func (sm *OpsyRunner) GetCurrentState(id string) amiv1alpha1.PackerBuilderState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	state, exists := sm.states[id]
	if !exists {
		return amiv1alpha1.StateInitial
	}
	return state
}

func (sm *OpsyRunner) isValidTransition(currentState, newState amiv1alpha1.PackerBuilderState) bool {
	validStates, exists := sm.transitions[currentState]
	if !exists {
		return false
	}
	for _, state := range validStates {
		if state == newState {
			return true
		}
	}
	return false
}
