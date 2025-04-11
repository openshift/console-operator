//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by codegen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackupJobReference) DeepCopyInto(out *BackupJobReference) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackupJobReference.
func (in *BackupJobReference) DeepCopy() *BackupJobReference {
	if in == nil {
		return nil
	}
	out := new(BackupJobReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionOperator) DeepCopyInto(out *ClusterVersionOperator) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionOperator.
func (in *ClusterVersionOperator) DeepCopy() *ClusterVersionOperator {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionOperator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterVersionOperator) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionOperatorList) DeepCopyInto(out *ClusterVersionOperatorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterVersionOperator, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionOperatorList.
func (in *ClusterVersionOperatorList) DeepCopy() *ClusterVersionOperatorList {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionOperatorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterVersionOperatorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionOperatorSpec) DeepCopyInto(out *ClusterVersionOperatorSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionOperatorSpec.
func (in *ClusterVersionOperatorSpec) DeepCopy() *ClusterVersionOperatorSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionOperatorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterVersionOperatorStatus) DeepCopyInto(out *ClusterVersionOperatorStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterVersionOperatorStatus.
func (in *ClusterVersionOperatorStatus) DeepCopy() *ClusterVersionOperatorStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterVersionOperatorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DelegatedAuthentication) DeepCopyInto(out *DelegatedAuthentication) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DelegatedAuthentication.
func (in *DelegatedAuthentication) DeepCopy() *DelegatedAuthentication {
	if in == nil {
		return nil
	}
	out := new(DelegatedAuthentication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DelegatedAuthorization) DeepCopyInto(out *DelegatedAuthorization) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DelegatedAuthorization.
func (in *DelegatedAuthorization) DeepCopy() *DelegatedAuthorization {
	if in == nil {
		return nil
	}
	out := new(DelegatedAuthorization)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EtcdBackup) DeepCopyInto(out *EtcdBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EtcdBackup.
func (in *EtcdBackup) DeepCopy() *EtcdBackup {
	if in == nil {
		return nil
	}
	out := new(EtcdBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EtcdBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EtcdBackupList) DeepCopyInto(out *EtcdBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]EtcdBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EtcdBackupList.
func (in *EtcdBackupList) DeepCopy() *EtcdBackupList {
	if in == nil {
		return nil
	}
	out := new(EtcdBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *EtcdBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EtcdBackupSpec) DeepCopyInto(out *EtcdBackupSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EtcdBackupSpec.
func (in *EtcdBackupSpec) DeepCopy() *EtcdBackupSpec {
	if in == nil {
		return nil
	}
	out := new(EtcdBackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EtcdBackupStatus) DeepCopyInto(out *EtcdBackupStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.BackupJob != nil {
		in, out := &in.BackupJob, &out.BackupJob
		*out = new(BackupJobReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EtcdBackupStatus.
func (in *EtcdBackupStatus) DeepCopy() *EtcdBackupStatus {
	if in == nil {
		return nil
	}
	out := new(EtcdBackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenerationHistory) DeepCopyInto(out *GenerationHistory) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenerationHistory.
func (in *GenerationHistory) DeepCopy() *GenerationHistory {
	if in == nil {
		return nil
	}
	out := new(GenerationHistory)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenericOperatorConfig) DeepCopyInto(out *GenericOperatorConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ServingInfo.DeepCopyInto(&out.ServingInfo)
	out.LeaderElection = in.LeaderElection
	out.Authentication = in.Authentication
	out.Authorization = in.Authorization
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenericOperatorConfig.
func (in *GenericOperatorConfig) DeepCopy() *GenericOperatorConfig {
	if in == nil {
		return nil
	}
	out := new(GenericOperatorConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GenericOperatorConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageContentSourcePolicy) DeepCopyInto(out *ImageContentSourcePolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageContentSourcePolicy.
func (in *ImageContentSourcePolicy) DeepCopy() *ImageContentSourcePolicy {
	if in == nil {
		return nil
	}
	out := new(ImageContentSourcePolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ImageContentSourcePolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageContentSourcePolicyList) DeepCopyInto(out *ImageContentSourcePolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ImageContentSourcePolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageContentSourcePolicyList.
func (in *ImageContentSourcePolicyList) DeepCopy() *ImageContentSourcePolicyList {
	if in == nil {
		return nil
	}
	out := new(ImageContentSourcePolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ImageContentSourcePolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageContentSourcePolicySpec) DeepCopyInto(out *ImageContentSourcePolicySpec) {
	*out = *in
	if in.RepositoryDigestMirrors != nil {
		in, out := &in.RepositoryDigestMirrors, &out.RepositoryDigestMirrors
		*out = make([]RepositoryDigestMirrors, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageContentSourcePolicySpec.
func (in *ImageContentSourcePolicySpec) DeepCopy() *ImageContentSourcePolicySpec {
	if in == nil {
		return nil
	}
	out := new(ImageContentSourcePolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoggingConfig) DeepCopyInto(out *LoggingConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoggingConfig.
func (in *LoggingConfig) DeepCopy() *LoggingConfig {
	if in == nil {
		return nil
	}
	out := new(LoggingConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodeStatus) DeepCopyInto(out *NodeStatus) {
	*out = *in
	if in.LastFailedDeploymentErrors != nil {
		in, out := &in.LastFailedDeploymentErrors, &out.LastFailedDeploymentErrors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NodeStatus.
func (in *NodeStatus) DeepCopy() *NodeStatus {
	if in == nil {
		return nil
	}
	out := new(NodeStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OLM) DeepCopyInto(out *OLM) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OLM.
func (in *OLM) DeepCopy() *OLM {
	if in == nil {
		return nil
	}
	out := new(OLM)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OLM) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OLMList) DeepCopyInto(out *OLMList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]OLM, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OLMList.
func (in *OLMList) DeepCopy() *OLMList {
	if in == nil {
		return nil
	}
	out := new(OLMList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *OLMList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OLMSpec) DeepCopyInto(out *OLMSpec) {
	*out = *in
	in.OperatorSpec.DeepCopyInto(&out.OperatorSpec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OLMSpec.
func (in *OLMSpec) DeepCopy() *OLMSpec {
	if in == nil {
		return nil
	}
	out := new(OLMSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OLMStatus) DeepCopyInto(out *OLMStatus) {
	*out = *in
	in.OperatorStatus.DeepCopyInto(&out.OperatorStatus)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OLMStatus.
func (in *OLMStatus) DeepCopy() *OLMStatus {
	if in == nil {
		return nil
	}
	out := new(OLMStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OperatorCondition) DeepCopyInto(out *OperatorCondition) {
	*out = *in
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OperatorCondition.
func (in *OperatorCondition) DeepCopy() *OperatorCondition {
	if in == nil {
		return nil
	}
	out := new(OperatorCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OperatorSpec) DeepCopyInto(out *OperatorSpec) {
	*out = *in
	out.Logging = in.Logging
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OperatorSpec.
func (in *OperatorSpec) DeepCopy() *OperatorSpec {
	if in == nil {
		return nil
	}
	out := new(OperatorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OperatorStatus) DeepCopyInto(out *OperatorStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]OperatorCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.CurrentAvailability != nil {
		in, out := &in.CurrentAvailability, &out.CurrentAvailability
		*out = new(VersionAvailability)
		(*in).DeepCopyInto(*out)
	}
	if in.TargetAvailability != nil {
		in, out := &in.TargetAvailability, &out.TargetAvailability
		*out = new(VersionAvailability)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OperatorStatus.
func (in *OperatorStatus) DeepCopy() *OperatorStatus {
	if in == nil {
		return nil
	}
	out := new(OperatorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RepositoryDigestMirrors) DeepCopyInto(out *RepositoryDigestMirrors) {
	*out = *in
	if in.Mirrors != nil {
		in, out := &in.Mirrors, &out.Mirrors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RepositoryDigestMirrors.
func (in *RepositoryDigestMirrors) DeepCopy() *RepositoryDigestMirrors {
	if in == nil {
		return nil
	}
	out := new(RepositoryDigestMirrors)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StaticPodOperatorStatus) DeepCopyInto(out *StaticPodOperatorStatus) {
	*out = *in
	in.OperatorStatus.DeepCopyInto(&out.OperatorStatus)
	if in.NodeStatuses != nil {
		in, out := &in.NodeStatuses, &out.NodeStatuses
		*out = make([]NodeStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StaticPodOperatorStatus.
func (in *StaticPodOperatorStatus) DeepCopy() *StaticPodOperatorStatus {
	if in == nil {
		return nil
	}
	out := new(StaticPodOperatorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VersionAvailability) DeepCopyInto(out *VersionAvailability) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Generations != nil {
		in, out := &in.Generations, &out.Generations
		*out = make([]GenerationHistory, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VersionAvailability.
func (in *VersionAvailability) DeepCopy() *VersionAvailability {
	if in == nil {
		return nil
	}
	out := new(VersionAvailability)
	in.DeepCopyInto(out)
	return out
}
