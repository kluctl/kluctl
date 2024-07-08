package webui

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	cp "github.com/otiai10/copy"
	"os"
	"path/filepath"
	"sync/atomic"
)

type StaticWebuiBuilder struct {
	store results.ResultStore

	maxResults int
}

func NewStaticWebuiBuilder(store results.ResultStore, maxResults int) *StaticWebuiBuilder {
	ret := &StaticWebuiBuilder{
		store:      store,
		maxResults: maxResults,
	}
	return ret
}

func (swb *StaticWebuiBuilder) Build(ctx context.Context, path string) error {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	summaries, err := swb.store.ListCommandResultSummaries(results.ListResultSummariesOptions{})
	if err != nil {
		return err
	}

	kds, err := swb.store.ListKluctlDeployments()
	if err != nil {
		return err
	}

	filtered := make([]result.CommandResultSummary, 0, len(summaries))
	counts := map[ProjectTargetKey]int{}
	for _, rs := range summaries {
		key := ProjectTargetKey{
			Project: rs.ProjectKey,
			Target:  rs.TargetKey,
		}
		cnt := counts[key]
		if cnt >= swb.maxResults {
			continue
		}
		counts[key]++
		filtered = append(filtered, rs)
	}
	summaries = filtered

	err = os.MkdirAll(filepath.Join(tmpDir, "staticdata"), 0o700)
	if err != nil {
		return err
	}

	doneCnt := atomic.Int32{}
	buildStatusStr := func() string {
		return fmt.Sprintf("Retrieved %d of %d results", doneCnt.Load(), len(summaries))
	}
	st := status.Start(ctx, buildStatusStr())
	defer st.Failed()

	gh := utils.NewGoHelper(ctx, 4)
	for _, rs := range summaries {
		rs := rs
		gh.RunE(func() error {
			cr, err := swb.store.GetCommandResult(results.GetCommandResultOptions{
				Id: rs.Id,
			})

			var js string

			if err != nil {
				jerr := struct {
					Error string `json:"error"`
				}{
					Error: err.Error(),
				}
				j, err := yaml.WriteJsonString(jerr)
				if err != nil {
					return err
				}
				js = fmt.Sprintf("staticResults.set(\"%s\", %s)\n", rs.Id, j)
			} else {
				j, err := yaml.WriteJsonString(cr)
				if err != nil {
					return err
				}
				js = fmt.Sprintf("staticResults.set(\"%s\", %s)\n", rs.Id, j)
			}
			err = os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("staticdata/result-%s.js", rs.Id)), []byte(js), 0o600)
			if err != nil {
				return err
			}
			doneCnt.Add(1)
			st.Update(buildStatusStr())
			return nil
		})
	}
	gh.Wait()

	err = gh.ErrorOrNil()
	if err != nil {
		return err
	}
	st.Success()

	j, err := yaml.WriteJsonString(summaries)
	if err != nil {
		return err
	}
	j = `const staticSummaries=` + j
	err = os.WriteFile(filepath.Join(tmpDir, "staticdata/summaries.js"), []byte(j), 0o600)
	if err != nil {
		return err
	}

	var kds2 []map[string]any
	for _, kd := range kds {
		kds2 = append(kds2, map[string]any{
			"clusterId":  kd.ClusterId,
			"deployment": kd.Deployment,
		})
	}
	j, err = yaml.WriteJsonString(kds2)
	if err != nil {
		return err
	}
	j = `const staticKluctlDeployments=` + j
	err = os.WriteFile(filepath.Join(tmpDir, "staticdata/kluctldeployments.js"), []byte(j), 0o600)
	if err != nil {
		return err
	}

	j, err = yaml.WriteJsonString(GetShortNames())
	if err != nil {
		return err
	}
	j = `const staticShortNames=` + j
	err = os.WriteFile(filepath.Join(tmpDir, "staticdata/shortnames.js"), []byte(j), 0o600)
	if err != nil {
		return err
	}

	err = cp.Copy(".", tmpDir, cp.Options{
		FS:            uiFS,
		AddPermission: 0o666,
	})
	if err != nil {
		return err
	}

	staticbuildJsBytes, err := os.ReadFile(filepath.Join(tmpDir, "staticbuild.js"))
	if err != nil {
		return err
	}
	staticbuildJs := string(staticbuildJsBytes) + "\nisStaticBuild = true\n"
	err = os.WriteFile(filepath.Join(tmpDir, "staticbuild.js"), []byte(staticbuildJs), 0)
	if err != nil {
		return err
	}

	err = cp.Copy(tmpDir, path)
	if err != nil {
		return err
	}

	return nil
}
