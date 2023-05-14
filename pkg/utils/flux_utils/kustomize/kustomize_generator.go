/*
Copyright 2022 The Flux authors

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

package kustomize

import (
	"fmt"
	"sync"

	securefs "github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/kustomize/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// buildMutex protects against kustomize concurrent map read/write panic
var kustomizeBuildMutex sync.Mutex

// Secure Build wraps krusty.MakeKustomizer with the following settings:
//   - secure on-disk FS denying operations outside root
//   - load files from outside the kustomization dir path
//     (but not outside root)
//   - disable plugins except for the builtin ones
func SecureBuild(root, dirPath string, allowRemoteBases bool) (res resmap.ResMap, err error) {
	var fs filesys.FileSystem

	// Create secure FS for root with or without remote base support
	if allowRemoteBases {
		fs, err = securefs.MakeFsOnDiskSecureBuild(root)
		if err != nil {
			return nil, err
		}
	} else {
		fs, err = securefs.MakeFsOnDiskSecure(root)
		if err != nil {
			return nil, err
		}
	}
	return Build(fs, dirPath)
}

// Build wraps krusty.MakeKustomizer with the following settings:
// - load files from outside the kustomization.yaml root
// - disable plugins except for the builtin ones
func Build(fs filesys.FileSystem, dirPath string) (res resmap.ResMap, err error) {
	// temporary workaround for concurrent map read and map write bug
	// https://github.com/kubernetes-sigs/kustomize/issues/3659
	kustomizeBuildMutex.Lock()
	defer kustomizeBuildMutex.Unlock()

	// Kustomize tends to panic in unpredicted ways due to (accidental)
	// invalid object data; recover when this happens to ensure continuity of
	// operations
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from kustomize build panic: %v", r)
		}
	}()

	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}

	k := krusty.MakeKustomizer(buildOptions)
	return k.Run(fs, dirPath)
}
