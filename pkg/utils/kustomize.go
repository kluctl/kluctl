/*
Code in this file is copied from https://github.com/fluxcd/kustomize-controller/blob/main/controllers/kustomization_generator.go
*/

package utils

import (
	"fmt"
	securefs "github.com/fluxcd/pkg/kustomize/filesys"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sync"
)

// TODO: remove mutex when kustomize fixes the concurrent map read/write panic
var kustomizeBuildMutex sync.Mutex

// SecureBuildKustomization wraps krusty.MakeKustomizer with the following settings:
//  - secure on-disk FS denying operations outside root
//  - load files from outside the kustomization dir path
//    (but not outside root)
//  - disable plugins except for the builtin ones
func SecureBuildKustomization(root, dirPath string, allowRemoteBases bool) (_ resmap.ResMap, err error) {
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

	// Temporary workaround for concurrent map read and map write bug
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
