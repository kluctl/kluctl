package kluctl_project

import (
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type dynamicTargetInfo struct {
	baseTarget    *types.Target
	dir           string
	gitProject    *types.GitProject
	ref           *string
	refPattern    *string
	defaultBranch string
}

func (c *LoadedKluctlProject) loadTargets() error {
	status.Trace(c.ctx, "Loading targets")
	defer status.Trace(c.ctx, "Done loading targets")

	targetNames := make(map[string]bool)
	c.DynamicTargets = nil

	var targetInfos []*dynamicTargetInfo
	for _, baseTarget := range c.Config.Targets {
		l, err := c.prepareDynamicTargets(baseTarget)
		if err != nil {
			return err
		}
		targetInfos = append(targetInfos, l...)
	}

	err := c.cloneDynamicTargets(targetInfos)
	if err != nil {
		return err
	}

	for _, targetInfo := range targetInfos {
		target, err := c.buildDynamicTarget(targetInfo)
		if err != nil {
			// Only fail if non-dynamic targets fail to load
			if targetInfo.refPattern == nil {
				return err
			}
			status.Warning(c.ctx, "Failed to load dynamic target config for project: %v", err)
			continue
		}

		target, err = c.renderTarget(target)
		if err != nil {
			return err
		}

		if _, ok := targetNames[target.Name]; ok {
			status.Warning(c.ctx, "Duplicate target %s", target.Name)
		} else {
			targetNames[target.Name] = true
			c.DynamicTargets = append(c.DynamicTargets, &types.DynamicTarget{
				Target:         target,
				BaseTargetName: targetInfo.baseTarget.Name,
			})
		}
	}
	sort.SliceStable(c.DynamicTargets, func(i, j int) bool {
		return c.DynamicTargets[i].Target.Name < c.DynamicTargets[j].Target.Name
	})
	return nil
}

func (c *LoadedKluctlProject) renderTarget(target *types.Target) (*types.Target, error) {
	// Try rendering the target multiple times, until all values can be rendered successfully. This allows the target
	// to reference itself in complex ways. We'll also try loading the cluster vars in each iteration.

	var errors []error
	curTarget := target
	for i := 0; i < 10; i++ {
		varsCtx := vars.NewVarsCtx(c.J2)
		err := varsCtx.UpdateChildFromStruct("target", curTarget)
		if err != nil {
			return nil, err
		}

		if target.Cluster != nil {
			cc, err := c.LoadClusterConfig(*target.Cluster)
			if err == nil {
				err = varsCtx.UpdateChildFromStruct("cluster", cc.Cluster)
				if err != nil {
					return nil, err
				}
			}
		}

		var newTarget types.Target
		err = c.J2.RenderStruct(&newTarget, curTarget, varsCtx.Vars)
		if err == nil && reflect.DeepEqual(curTarget, &newTarget) {
			return curTarget, nil
		}
		curTarget = &newTarget
	}
	if len(errors) != 0 {
		return nil, errors[0]
	}
	return curTarget, nil
}

func (c *LoadedKluctlProject) prepareDynamicTargets(baseTarget *types.Target) ([]*dynamicTargetInfo, error) {
	if baseTarget.TargetConfig != nil && baseTarget.TargetConfig.Project != nil {
		return c.prepareDynamicTargetsExternal(baseTarget)
	} else {
		return c.prepareDynamicTargetsSimple(baseTarget)
	}
}

func (c *LoadedKluctlProject) prepareDynamicTargetsSimple(baseTarget *types.Target) ([]*dynamicTargetInfo, error) {
	if baseTarget.TargetConfig != nil {
		if baseTarget.TargetConfig.Ref != nil || baseTarget.TargetConfig.RefPattern != nil {
			return nil, fmt.Errorf("'ref' and/or 'refPattern' are not allowed for non-external dynamic targets")
		}
	}
	dynamicTargets := []*dynamicTargetInfo{
		{
			baseTarget: baseTarget,
			dir:        c.ProjectDir,
		},
	}
	return dynamicTargets, nil
}

func (c *LoadedKluctlProject) prepareDynamicTargetsExternal(baseTarget *types.Target) ([]*dynamicTargetInfo, error) {
	ge, err := c.RP.GetEntry(baseTarget.TargetConfig.Project.Url)
	if err != nil {
		return nil, err
	}

	repoInfo := ge.GetRepoInfo()

	if baseTarget.TargetConfig.Ref != nil && baseTarget.TargetConfig.RefPattern != nil {
		return nil, fmt.Errorf("'refPattern' and 'ref' can't be specified together")
	}

	targetConfigRef := baseTarget.TargetConfig.Ref
	refPattern := baseTarget.TargetConfig.RefPattern

	defaultBranch := repoInfo.DefaultRef
	if defaultBranch == "" {
		return nil, fmt.Errorf("git project %v seems to have no default branch", baseTarget.TargetConfig.Project.Url.String())
	}

	if baseTarget.TargetConfig.Ref == nil && baseTarget.TargetConfig.RefPattern == nil {
		// use default branch of repo
		targetConfigRef = &defaultBranch
	}

	refs := repoInfo.RemoteRefs
	if targetConfigRef != nil {
		if _, ok := refs[fmt.Sprintf("refs/heads/%s", *targetConfigRef)]; !ok {
			return nil, fmt.Errorf("git project %s has no ref %s", baseTarget.TargetConfig.Project.Url.String(), *targetConfigRef)
		}
		refPattern = targetConfigRef
	}

	var dynamicTargets []*dynamicTargetInfo
	for ref := range refs {
		m, refShortName, err := c.matchRef(ref, *refPattern)
		if err != nil {
			return nil, err
		}
		if !m {
			continue
		}

		ge, err := c.RP.GetEntry(baseTarget.TargetConfig.Project.Url)
		if err != nil {
			return nil, err
		}

		cloneDir, _, err := ge.GetClonedDir(refShortName)
		if err != nil {
			return nil, err
		}

		dynamicTargets = append(dynamicTargets, &dynamicTargetInfo{
			baseTarget:    baseTarget,
			dir:           cloneDir,
			gitProject:    baseTarget.TargetConfig.Project,
			ref:           &refShortName,
			refPattern:    refPattern,
			defaultBranch: defaultBranch,
		})
	}
	return dynamicTargets, nil
}

func (c *LoadedKluctlProject) matchRef(s string, pattern string) (bool, string, error) {
	if strings.HasPrefix(pattern, "refs/") {
		p, err := regexp.Compile(fmt.Sprintf("^%s$", pattern))
		if err != nil {
			return false, "", err
		}
		return p.MatchString(s), s, nil
	}
	p1, err := regexp.Compile(fmt.Sprintf("^refs/heads/%s$", pattern))
	if err != nil {
		return false, "", err
	}
	p2, err := regexp.Compile(fmt.Sprintf("^refs/tags/%s$", pattern))
	if err != nil {
		return false, "", err
	}
	if p1.MatchString(s) {
		return true, s[len("refs/heads/"):], nil
	} else if p2.MatchString(s) {
		return true, s[len("refs/tags/"):], nil
	} else {
		return false, "", nil
	}
}

func (c *LoadedKluctlProject) cloneDynamicTargets(dynamicTargets []*dynamicTargetInfo) error {
	uniqueClones := make(map[string]interface{})
	var mutex sync.Mutex
	var wg sync.WaitGroup

	for _, targetInfo_ := range dynamicTargets {
		targetInfo := targetInfo_

		if targetInfo.gitProject == nil {
			continue
		}
		mutex.Lock()
		if _, ok := uniqueClones[targetInfo.dir]; ok {
			mutex.Unlock()
			continue
		}
		uniqueClones[targetInfo.dir] = nil
		mutex.Unlock()

		wg.Add(1)
		go func() {
			defer wg.Done()
			gitProject := *targetInfo.gitProject
			gitProject.Ref = *targetInfo.ref

			gi, err := c.loadGitProject(&gitProject, "")
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				uniqueClones[targetInfo.dir] = err
			} else {
				uniqueClones[targetInfo.dir] = &gi
			}
		}()
	}
	wg.Wait()

	return nil
}

func (c *LoadedKluctlProject) loadTargetConfigFile(targetInfo *dynamicTargetInfo) ([]byte, error) {
	configFile := yaml.FixNameExt(targetInfo.dir, "target-config.yml")
	if targetInfo.baseTarget.TargetConfig.File != nil {
		configFile = *targetInfo.baseTarget.TargetConfig.File
	}
	configPath, err := securejoin.SecureJoin(targetInfo.dir, configFile)
	if err != nil {
		return nil, err
	}
	if !utils.IsFile(configPath) {
		return nil, fmt.Errorf("no target config file with name %s found in target", configFile)
	}

	return os.ReadFile(configPath)
}

func (c *LoadedKluctlProject) buildDynamicTarget(targetInfo *dynamicTargetInfo) (*types.Target, error) {
	var target types.Target
	err := utils.DeepCopy(&target, targetInfo.baseTarget)
	if err != nil {
		return nil, err
	}
	if targetInfo.baseTarget.TargetConfig == nil {
		return &target, nil
	}

	configFile, err := c.loadTargetConfigFile(targetInfo)
	if err != nil {
		return nil, err
	}

	var targetConfig types.TargetConfig
	err = yaml.ReadYamlBytes(configFile, &targetConfig)
	if err != nil {
		return nil, err
	}

	// check and merge args
	if targetConfig.Args != nil {
		err = targetConfig.Args.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
			strValue := fmt.Sprintf("%v", it.Value())
			err := c.CheckDynamicArg(&target, it.KeyPath().ToJsonPath(), strValue)
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		target.Args.Merge(targetConfig.Args)
	}
	// We prepend the dynamic images to ensure they get higher priority later
	target.Images = append(targetConfig.Images, target.Images...)

	if targetInfo.ref != nil {
		target.TargetConfig.Ref = targetInfo.ref
	}

	return &target, nil
}
