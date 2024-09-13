//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AMIFilters) DeepCopyInto(out *AMIFilters) {
	*out = *in
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AMIFilters.
func (in *AMIFilters) DeepCopy() *AMIFilters {
	if in == nil {
		return nil
	}
	out := new(AMIFilters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AMIRefresher) DeepCopyInto(out *AMIRefresher) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AMIRefresher.
func (in *AMIRefresher) DeepCopy() *AMIRefresher {
	if in == nil {
		return nil
	}
	out := new(AMIRefresher)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AMIRefresher) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AMIRefresherList) DeepCopyInto(out *AMIRefresherList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AMIRefresher, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AMIRefresherList.
func (in *AMIRefresherList) DeepCopy() *AMIRefresherList {
	if in == nil {
		return nil
	}
	out := new(AMIRefresherList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AMIRefresherList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AMIRefresherSpec) DeepCopyInto(out *AMIRefresherSpec) {
	*out = *in
	if in.AMIFilters != nil {
		in, out := &in.AMIFilters, &out.AMIFilters
		*out = make([]AMIFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Exclude != nil {
		in, out := &in.Exclude, &out.Exclude
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	out.TimeOuts = in.TimeOuts
	in.RefreshConfig.DeepCopyInto(&out.RefreshConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AMIRefresherSpec.
func (in *AMIRefresherSpec) DeepCopy() *AMIRefresherSpec {
	if in == nil {
		return nil
	}
	out := new(AMIRefresherSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AMIRefresherStatus) DeepCopyInto(out *AMIRefresherStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]*OpsyCondition, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(OpsyCondition)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	in.LastRun.DeepCopyInto(&out.LastRun)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AMIRefresherStatus.
func (in *AMIRefresherStatus) DeepCopy() *AMIRefresherStatus {
	if in == nil {
		return nil
	}
	out := new(AMIRefresherStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Builder) DeepCopyInto(out *Builder) {
	*out = *in
	if in.Commands != nil {
		in, out := &in.Commands, &out.Commands
		*out = make([]Command, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Builder.
func (in *Builder) DeepCopy() *Builder {
	if in == nil {
		return nil
	}
	out := new(Builder)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Command) DeepCopyInto(out *Command) {
	*out = *in
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Command.
func (in *Command) DeepCopy() *Command {
	if in == nil {
		return nil
	}
	out := new(Command)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Notify) DeepCopyInto(out *Notify) {
	*out = *in
	in.Slack.DeepCopyInto(&out.Slack)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Notify.
func (in *Notify) DeepCopy() *Notify {
	if in == nil {
		return nil
	}
	out := new(Notify)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpsyCondition) DeepCopyInto(out *OpsyCondition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpsyCondition.
func (in *OpsyCondition) DeepCopy() *OpsyCondition {
	if in == nil {
		return nil
	}
	out := new(OpsyCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackerBuilder) DeepCopyInto(out *PackerBuilder) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackerBuilder.
func (in *PackerBuilder) DeepCopy() *PackerBuilder {
	if in == nil {
		return nil
	}
	out := new(PackerBuilder)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PackerBuilder) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackerBuilderList) DeepCopyInto(out *PackerBuilderList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PackerBuilder, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackerBuilderList.
func (in *PackerBuilderList) DeepCopy() *PackerBuilderList {
	if in == nil {
		return nil
	}
	out := new(PackerBuilderList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PackerBuilderList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackerBuilderSpec) DeepCopyInto(out *PackerBuilderSpec) {
	*out = *in
	if in.AMIFilters != nil {
		in, out := &in.AMIFilters, &out.AMIFilters
		*out = make([]AMIFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.TimeOuts = in.TimeOuts
	out.GitSync = in.GitSync
	in.Notify.DeepCopyInto(&out.Notify)
	if in.MaxNumberOfJobs != nil {
		in, out := &in.MaxNumberOfJobs, &out.MaxNumberOfJobs
		*out = new(int32)
		**out = **in
	}
	in.Builder.DeepCopyInto(&out.Builder)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackerBuilderSpec.
func (in *PackerBuilderSpec) DeepCopy() *PackerBuilderSpec {
	if in == nil {
		return nil
	}
	out := new(PackerBuilderSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackerBuilderStatus) DeepCopyInto(out *PackerBuilderStatus) {
	*out = *in
	if in.Active != nil {
		in, out := &in.Active, &out.Active
		*out = make([]v1.ObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]*OpsyCondition, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(OpsyCondition)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.FailedJobCount != nil {
		in, out := &in.FailedJobCount, &out.FailedJobCount
		*out = new(int32)
		**out = **in
	}
	in.LastRun.DeepCopyInto(&out.LastRun)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackerBuilderStatus.
func (in *PackerBuilderStatus) DeepCopy() *PackerBuilderStatus {
	if in == nil {
		return nil
	}
	out := new(PackerBuilderStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RefreshConfig) DeepCopyInto(out *RefreshConfig) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(int32)
		**out = **in
	}
	if in.MaxUnavailablePercentage != nil {
		in, out := &in.MaxUnavailablePercentage, &out.MaxUnavailablePercentage
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RefreshConfig.
func (in *RefreshConfig) DeepCopy() *RefreshConfig {
	if in == nil {
		return nil
	}
	out := new(RefreshConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SlackNotifier) DeepCopyInto(out *SlackNotifier) {
	*out = *in
	if in.Channels != nil {
		in, out := &in.Channels, &out.Channels
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SlackNotifier.
func (in *SlackNotifier) DeepCopy() *SlackNotifier {
	if in == nil {
		return nil
	}
	out := new(SlackNotifier)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Sync) DeepCopyInto(out *Sync) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Sync.
func (in *Sync) DeepCopy() *Sync {
	if in == nil {
		return nil
	}
	out := new(Sync)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeOuts) DeepCopyInto(out *TimeOuts) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeOuts.
func (in *TimeOuts) DeepCopy() *TimeOuts {
	if in == nil {
		return nil
	}
	out := new(TimeOuts)
	in.DeepCopyInto(out)
	return out
}