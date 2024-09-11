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

package v1alpha1

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type JobStatus string
type OnErrorBehavior string

type ImageType string
type PackerBuilderState string

const (
	StateInitial      PackerBuilderState = "Initial"
	StateFlightChecks PackerBuilderState = "FlightChecks"
	StateJobCreation  PackerBuilderState = "JobCreation"
	StateJobRunning   PackerBuilderState = "JobRunning"
	StateJobCompleted PackerBuilderState = "JobCompleted"
	StateJobFailed    PackerBuilderState = "JobFailed"
	StateAMICreated   PackerBuilderState = "AMICreated"
	StateError        PackerBuilderState = "Error"
)

const (
	AMI       ImageType = "ami"
	Container ImageType = "container"
)

const (
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusRunning   JobStatus = "running"
)

const (
	OnErrorCleanup OnErrorBehavior = "cleanup"
	OnErrorAbort   OnErrorBehavior = "abort"
	OnErrorIgnore  OnErrorBehavior = "ignore"
)

type Notify struct {
	//+optional
	Slack SlackNotifier `json:"slack,omitempty"`
	// SNS   SNSNotifier   `json:"sns,omitempty"`
	// SES   SESNotifier   `json:"ses,omitempty"`
}

type SlackNotifier struct {
	Channels []string `json:"channelIDs,omitempty"`
	Secret   string   `json:"secret,omitempty"`
}

type PackerBuilderSpec struct {
	//+optional
	AMIFilters []AMIFilters `json:"amiFilters,omitempty"`
	//+optional
	ClusterName string `json:"clusterName,omitempty"`
	//+optional
	TimeOuts TimeOuts `json:"timeOuts,omitempty"`
	//+optional
	GitSync Sync `json:"gitSync,omitempty"`

	Notify Notify `json:"notifier,omitempty"`

	//+optional
	MaxNumberOfJobs *int32 `json:"maxNumberOfJobs,omitempty"`
	//+optional
	Region string `json:"region,omitempty"`
	//+optional
	Builder Builder `json:"builder,omitempty"`
}

type Sync struct {
	Image  string `json:"image,omitempty"`
	Name   string `json:"name,omitempty"`
	Secret string `json:"secret,omitempty"`
}

type Builder struct {
	Branch    string    `json:"branch,omitempty"`
	Commands  []Command `json:"commands,omitempty"`
	Debug     bool      `json:"debug,omitempty"`
	Dir       string    `json:"dir,omitempty"`
	Image     string    `json:"image,omitempty"`
	ImageType ImageType `json:"imageType,omitempty"`
	RepoURL   string    `json:"repoURL,omitempty"`
	Secret    string    `json:"secret,omitempty"`
}

type Command struct {
	Args          []string          `json:"args,omitempty"`
	Color         bool              `json:"color,omitempty"`
	Debug         bool              `json:"debug,omitempty"`
	OnError       OnErrorBehavior   `json:"onError,omitempty"`
	SubCommand    string            `json:"subCommand,omitempty"`
	Variables     map[string]string `json:"variables,omitempty"`
	VariablesFile string            `json:"variablesFile,omitempty"`
	WorkingDir    string            `json:"workingDir,omitempty"`
}

type PackerBuilderStatus struct {
	//+optional
	Active []corev1.ObjectReference `json:"active,omitempty"`
	//+optional
	BuildID string `json:"buildID,omitempty"`
	//+optional
	Command string `json:"command,omitempty"`
	//+optional
	Conditions []*OpsyCondition `json:"conditions,omitempty"`
	//+optional
	FailedJobCount *int32 `json:"failedJobCount,omitempty"`
	//+optional
	JobID string `json:"jobID,omitempty"`
	//+optional
	JobName string `json:"jobName,omitempty"`
	//+optional
	JobStatus JobStatus `json:"jobStatus,omitempty"`
	//+optional
	LastRun metav1.Time `json:"lastRun,omitempty"`
	//+optional
	LastRunBaseImageID string `json:"lastRunBaseImageID,omitempty"`
	//+optional
	LastRunBuiltImageID string `json:"lastRunBuiltImageID,omitempty"`
	//+optional
	LastRunMessage string `json:"lastRunMessage,omitempty"`
	//+optional
	LastRunStatus LastRunStatus `json:"lastRunStatus,omitempty"`
	//+optional
	LastRunSecurityGroupID string `json:"lastRunSecurityGroupID,omitempty"`
	//+optional
	LastRunKeyPair string `json:"lastRunKeyPair,omitempty"`
	//+optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	//+optional
	State PackerBuilderState `json:"state,omitempty"`
	// Notify Notify `json:"notify,omitempty"`
	//+optional

}

// +kubebuilder:resource:shortName=pb
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Base Image",type="string",JSONPath=".status.lastRunBaseImageID"
// +kubebuilder:printcolumn:name="Built Image",type="string",JSONPath=".status.lastRunBuiltImageID"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type PackerBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackerBuilderSpec   `json:"spec,omitempty"`
	Status PackerBuilderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type PackerBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackerBuilder `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PackerBuilder{}, &PackerBuilderList{})
}

func (pb *PackerBuilder) GetConditions() []*OpsyCondition {
	return pb.Status.Conditions
}

func (pb *PackerBuilder) SetConditions(conditions []*OpsyCondition) {
	pb.Status.Conditions = conditions
}

func (pb *PackerBuilder) GetControllerTimer() string {
	return pb.Spec.TimeOuts.ControllerTimer
}

func (pc *Command) String() string {
	var parts []string
	parts = append(parts, "packer", pc.SubCommand)
	parts = append(parts, pc.Args...)

	if pc.SubCommand == "build" {
		parts = append(parts, pc.buildFlags()...)
	}

	if pc.WorkingDir != "" {
		parts = append(parts, pc.WorkingDir)
	}

	return strings.Join(parts, " ")
}

func (pc *Command) buildFlags() []string {
	var flags []string

	for key, value := range pc.Variables {
		flags = append(flags, fmt.Sprintf("-var '%s=%s'", key, value))
	}

	if pc.VariablesFile != "" {
		flags = append(flags, "-var-file="+pc.VariablesFile)
	}

	if pc.Debug {
		flags = append(flags, "-debug")
	}

	if pc.OnError != "" {
		flags = append(flags, "-on-error="+string(pc.OnError))
	}

	if !pc.Color {
		flags = append(flags, "-color=false")
	}

	return flags
}
