package webui

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	cp "github.com/otiai10/copy"
	"os"
	"path/filepath"
)

type StaticWebuiBuilder struct {
	store results.ResultStore
}

func NewStaticWebuiBuilder(store results.ResultStore) *StaticWebuiBuilder {
	ret := &StaticWebuiBuilder{
		store: store,
	}
	return ret
}

func (swb *StaticWebuiBuilder) Build(path string) error {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	summaries, err := swb.store.ListCommandResultSummaries(results.ListCommandResultSummariesOptions{})
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Join(tmpDir, "staticdata"), 0o700)
	if err != nil {
		return err
	}

	gh := utils.NewGoHelper(context.Background(), 8)
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
			return nil
		})
	}
	gh.Wait()

	j, err := yaml.WriteJsonString(summaries)
	if err != nil {
		return err
	}
	j = `const staticSummaries=` + j
	err = os.WriteFile(filepath.Join(tmpDir, "staticdata/summaries.js"), []byte(j), 0o600)
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

	err = utils.FsCopyDir(uiFS, ".", tmpDir)
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
