package vars

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getsops/sops/v3/cmd/sops/formats"
	"github.com/gobwas/glob"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/runtime"
)

func (v *VarsLoader) loadGitFiles(ctx context.Context, varsCtx *VarsCtx, gitFiles *types.VarsSourceGitFiles, ignoreMissing bool) ([]*uo.UnstructuredObject, bool, error) {
	sensible := false

	ge, err := v.rp.GetEntry(gitFiles.Url.String())
	if err != nil {
		return nil, false, err
	}

	matchingRefs, err := v.filterGitRefs(gitFiles.Ref, ge)
	if err != nil {
		return nil, false, err
	}

	if len(matchingRefs) == 0 {
		if ignoreMissing {
			return nil, false, nil
		}
	}

	var globs []glob.Glob
	for _, f := range gitFiles.Files {
		g, err := glob.Compile(f.Glob, '/')
		if err != nil {
			return nil, false, err
		}
		globs = append(globs, g)
	}

	results := make([]*uo.UnstructuredObject, 0, len(matchingRefs))
	for ref, hash := range matchingRefs {
		dir, _, err := ge.GetClonedDir(&gittypes.GitRef{
			Commit: hash,
		})
		if err != nil {
			return nil, false, err
		}

		var matchedFiles []types.GitFileMatch

		err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			var gitFile *types.GitFile
			for i, g := range globs {
				if g.Match(filepath.ToSlash(relPath)) {
					gitFile = &gitFiles.Files[i]
					break
				}
			}
			if gitFile == nil {
				return nil
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(contentBytes)
			size := int32(info.Size())

			if gitFile.Render {
				x, err := varsCtx.RenderString(content, nil)
				if err != nil {
					return fmt.Errorf("failed to render %s: %w", relPath, err)
				}
				content = x
			}

			format := formats.FormatForPath(path)
			decrypted, isEncrypted, err := sops.MaybeDecrypt(v.sops, []byte(content), format, format)
			if err != nil {
				return err
			}
			if isEncrypted {
				sensible = true
				content = string(decrypted)
				size = int32(len(decrypted))
			}

			var parsedRawExt *runtime.RawExtension
			if gitFile.ParseYaml {
				var parsed any
				if gitFile.YamlMultiDoc {
					parsed, err = yaml.ReadYamlAllString(content)
				} else {
					err = yaml.ReadYamlString(content, &parsed)
				}
				if err != nil {
					return fmt.Errorf("failed to parse %s: %w", relPath, err)
				}
				x, err := yaml.WriteJsonString(parsed)
				if err != nil {
					return fmt.Errorf("failed to parse %s: %w", relPath, err)
				}
				parsedRawExt = &runtime.RawExtension{Raw: []byte(x)}
			}

			matchedFiles = append(matchedFiles, types.GitFileMatch{
				File:    *gitFile,
				Path:    filepath.ToSlash(relPath),
				Size:    size,
				Content: content,
				Parsed:  parsedRawExt,
			})

			return nil
		})
		if err != nil {
			return nil, false, err
		}

		filesByPath := map[string]types.GitFileMatch{}
		filesTree := uo.New()
		for _, gf := range matchedFiles {
			filesByPath[gf.Path] = gf
			keys := strings.Split(gf.Path, "/")
			keysI := make([]any, 0, len(keys))
			for _, k := range keys {
				keysI = append(keysI, k)
			}
			err := filesTree.SetNestedField(gf, keysI...)
			if err != nil {
				return nil, false, err
			}
		}

		result := types.GitFilesRefMatch{
			Ref:         ref,
			RefStr:      ref.String(),
			Files:       matchedFiles,
			FilesByPath: filesByPath,
			FilesTree:   filesTree,
		}

		o, err := uo.FromStruct(result)
		if err != nil {
			return nil, false, err
		}
		results = append(results, o)
	}

	return results, sensible, nil
}

func (v *VarsLoader) filterGitRefs(ref *gittypes.GitRef, ge *repocache.GitCacheEntry) (map[gittypes.GitRef]string, error) {
	var err error
	matchingRefs := map[gittypes.GitRef]string{}

	ri := ge.GetRepoInfo()

	if ref == nil {
		defaultRef := ri.DefaultRef.String()
		hash, ok := ri.RemoteRefs[defaultRef]
		if !ok {
			return nil, fmt.Errorf("default ref %s not found", defaultRef)
		}
		matchingRefs[ri.DefaultRef] = hash
		return matchingRefs, nil
	}

	if ref.Commit != "" {
		found := false
		for name, hash := range ri.RemoteRefs {
			if hash == ref.Commit {
				ref2, err := gittypes.ParseGitRef(name)
				if err != nil {
					return nil, err
				}
				matchingRefs[ref2] = hash
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("commit %s not found", ref.Commit)
		}
		return matchingRefs, nil
	}

	var regex *regexp.Regexp
	if ref.Tag != "" {
		regex, err = regexp.Compile(fmt.Sprintf("^refs/tags/%s$", ref.Tag))
		if err != nil {
			return nil, fmt.Errorf("invalid tag regex specified: %w", err)
		}
	} else if ref.Branch != "" {
		regex, err = regexp.Compile(fmt.Sprintf("^refs/heads/%s$", ref.Branch))
		if err != nil {
			return nil, fmt.Errorf("invalid branch regex specified: %w", err)
		}
	} else {
		return nil, fmt.Errorf("ref is empty")
	}

	for name, hash := range ri.RemoteRefs {
		if regex.MatchString(name) {
			ref2, err := gittypes.ParseGitRef(name)
			if err != nil {
				return nil, err
			}
			matchingRefs[ref2] = hash
		}
	}

	return matchingRefs, nil
}
