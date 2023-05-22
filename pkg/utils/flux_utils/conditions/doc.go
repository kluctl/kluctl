/*
Copyright 2021 The Flux authors

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

// Package conditions provides utilities for manipulating the status conditions of Kubernetes resource objects that
// implement the Getter and/or Setter interfaces.
//
// Usage of this package within GitOps Toolkit components working with conditions is RECOMMENDED, as it provides a wide
// range of helpers to work around common reconciler problems, like setting a Condition status based on a
// summarization of other conditions, producing an aggregate Condition based on the conditions of a list of Kubernetes
// resources objects, recognition of negative polarity or "abnormal-true" conditions, etc.
package conditions
