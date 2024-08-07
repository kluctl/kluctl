//go:build !ignore_autogenerated

/*
Copyright 2023.

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

package types

import ()

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitInfo) DeepCopyInto(out *GitInfo) {
	*out = *in
	if in.Url != nil {
		in, out := &in.Url, &out.Url
		*out = (*in).DeepCopy()
	}
	if in.Ref != nil {
		in, out := &in.Ref, &out.Ref
		*out = new(GitRef)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitInfo.
func (in *GitInfo) DeepCopy() *GitInfo {
	if in == nil {
		return nil
	}
	out := new(GitInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GitRef) DeepCopyInto(out *GitRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitRef.
func (in *GitRef) DeepCopy() *GitRef {
	if in == nil {
		return nil
	}
	out := new(GitRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GitUrl.
func (in *GitUrl) DeepCopy() *GitUrl {
	if in == nil {
		return nil
	}
	out := new(GitUrl)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ProjectKey) DeepCopyInto(out *ProjectKey) {
	*out = *in
	out.RepoKey = in.RepoKey
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ProjectKey.
func (in *ProjectKey) DeepCopy() *ProjectKey {
	if in == nil {
		return nil
	}
	out := new(ProjectKey)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RepoKey) DeepCopyInto(out *RepoKey) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RepoKey.
func (in *RepoKey) DeepCopy() *RepoKey {
	if in == nil {
		return nil
	}
	out := new(RepoKey)
	in.DeepCopyInto(out)
	return out
}
