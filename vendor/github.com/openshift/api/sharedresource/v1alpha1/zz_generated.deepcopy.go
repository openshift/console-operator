//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by codegen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedConfigMap) DeepCopyInto(out *SharedConfigMap) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedConfigMap.
func (in *SharedConfigMap) DeepCopy() *SharedConfigMap {
	if in == nil {
		return nil
	}
	out := new(SharedConfigMap)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SharedConfigMap) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedConfigMapList) DeepCopyInto(out *SharedConfigMapList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SharedConfigMap, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedConfigMapList.
func (in *SharedConfigMapList) DeepCopy() *SharedConfigMapList {
	if in == nil {
		return nil
	}
	out := new(SharedConfigMapList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SharedConfigMapList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedConfigMapReference) DeepCopyInto(out *SharedConfigMapReference) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedConfigMapReference.
func (in *SharedConfigMapReference) DeepCopy() *SharedConfigMapReference {
	if in == nil {
		return nil
	}
	out := new(SharedConfigMapReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedConfigMapSpec) DeepCopyInto(out *SharedConfigMapSpec) {
	*out = *in
	out.ConfigMapRef = in.ConfigMapRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedConfigMapSpec.
func (in *SharedConfigMapSpec) DeepCopy() *SharedConfigMapSpec {
	if in == nil {
		return nil
	}
	out := new(SharedConfigMapSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedConfigMapStatus) DeepCopyInto(out *SharedConfigMapStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedConfigMapStatus.
func (in *SharedConfigMapStatus) DeepCopy() *SharedConfigMapStatus {
	if in == nil {
		return nil
	}
	out := new(SharedConfigMapStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedSecret) DeepCopyInto(out *SharedSecret) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedSecret.
func (in *SharedSecret) DeepCopy() *SharedSecret {
	if in == nil {
		return nil
	}
	out := new(SharedSecret)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SharedSecret) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedSecretList) DeepCopyInto(out *SharedSecretList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SharedSecret, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedSecretList.
func (in *SharedSecretList) DeepCopy() *SharedSecretList {
	if in == nil {
		return nil
	}
	out := new(SharedSecretList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SharedSecretList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedSecretReference) DeepCopyInto(out *SharedSecretReference) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedSecretReference.
func (in *SharedSecretReference) DeepCopy() *SharedSecretReference {
	if in == nil {
		return nil
	}
	out := new(SharedSecretReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedSecretSpec) DeepCopyInto(out *SharedSecretSpec) {
	*out = *in
	out.SecretRef = in.SecretRef
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedSecretSpec.
func (in *SharedSecretSpec) DeepCopy() *SharedSecretSpec {
	if in == nil {
		return nil
	}
	out := new(SharedSecretSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SharedSecretStatus) DeepCopyInto(out *SharedSecretStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SharedSecretStatus.
func (in *SharedSecretStatus) DeepCopy() *SharedSecretStatus {
	if in == nil {
		return nil
	}
	out := new(SharedSecretStatus)
	in.DeepCopyInto(out)
	return out
}
