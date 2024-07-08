package vars

import (
	"context"
	"github.com/getsops/sops/v3/age"
	git2 "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	gittypes "github.com/kluctl/kluctl/v2/lib/git/types"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	"github.com/kluctl/kluctl/v2/pkg/clouds/gcp"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

func (s *VarsLoaderTestSuite) TestGitFiles() {
	gs := test_utils.NewTestGitServer(s.T())
	gs.GitInit("repo")

	wt := gs.GetWorktree("repo")

	updateFile := func(name string, v int) {
		gs.UpdateYaml("repo", name, func(o map[string]any) error {
			o["test1"] = map[string]any{
				"test2": v,
			}
			return nil
		}, "")
	}

	updateFile("test1.yaml", 42)
	updateFile("test2.yaml", 43)
	updateFile("dir1/test.yaml", 1)
	updateFile("dir2/test.yaml", 2)

	err := wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("b1"),
		Create: true,
	})
	assert.NoError(s.T(), err)

	updateFile("test1.yaml", 142)
	updateFile("test2.yaml", 143)

	err = wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.Master,
	})
	assert.NoError(s.T(), err)

	err = wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("b2"),
		Create: true,
	})
	assert.NoError(s.T(), err)

	updateFile("test1.yaml", 242)
	updateFile("test2.yaml", 243)

	err = wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("c1"),
		Create: true,
	})
	assert.NoError(s.T(), err)

	gs.DeleteFile("repo", "test1.yaml", "")
	gs.DeleteFile("repo", "test2.yaml", "")
	updateFile("test3.yaml", 342)
	updateFile("test4.yaml", 343)
	gs.UpdateFile("repo", "multidoc.yaml", func(f string) (string, error) {
		return `
test1:
  test2: 1000
---
test1:
  test2: 2000
`, nil
	}, "")
	gs.UpdateFile("repo", "rendered.yaml", func(f string) (string, error) {
		return `
test1:
  test2: {{ 1 + 2 }}
---
test1:
  test2: {{ 1 + 3 }}
`, nil
	}, "")

	gs.UpdateFile("repo", "sops.yaml", func(f string) (string, error) {
		x, _ := sops_test_resources.TestResources.ReadFile("test.yaml")
		return string(x), nil
	}, "")
	key, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	s.T().Setenv(age.SopsAgeKeyEnv, string(key))

	err = wt.Checkout(&git2.CheckoutOptions{
		Branch: plumbing.Master,
	})
	assert.NoError(s.T(), err)

	assertResults := func(vc *VarsCtx, targetPath string, expectedResults []types.GitFilesRefMatch) {
		targetPath2 := uo.NewMyJsonPathMust(targetPath)
		resultsO, ok, _ := targetPath2.GetFirstListOfObjects(vc.Vars)
		if !ok {
			s.T().Errorf("results not found")
		}
		var results []types.GitFilesRefMatch
		for _, ro := range resultsO {
			var x types.GitFilesRefMatch
			err := ro.ToStruct(&x)
			if err != nil {
				s.T().Fatal(err)
			}
			results = append(results, x)
		}
		if len(results) != len(expectedResults) {
			s.T().Errorf("expected num of results does not match: %d != %d", len(results), len(expectedResults))
			return
		}
		for _, er := range expectedResults {
			found := false
			for _, r := range results {
				if r.Ref == er.Ref {
					found = true

					if r.RefStr != er.Ref.String() {
						s.T().Errorf("refStr: %s != %s", r.RefStr, r.Ref.String())
						return
					}

					err := wt.Checkout(&git2.CheckoutOptions{
						Branch: plumbing.ReferenceName(r.Ref.String()),
					})
					assert.NoError(s.T(), err)

					assert.Len(s.T(), r.Files, len(er.Files))

					for _, ef := range er.Files {
						foundFile := false
						for _, f := range r.Files {
							if f.Path != ef.Path {
								continue
							}
							if f.File.Glob != ef.File.Glob {
								s.T().Errorf("glob does not match: %s != %s", f.File.Glob, ef.File.Glob)
							}
							foundFile = true

							if ef.Content == "" {
								expectedContent := gs.ReadFile("repo", f.Path)
								assert.Equal(s.T(), string(expectedContent), f.Content)
							} else {
								assert.Equal(s.T(), ef.Content, f.Content)
							}
							if f.File.ParseYaml {
								var parsed any
								if f.File.YamlMultiDoc {
									parsed, err = yaml.ReadYamlAllString(f.Content)
									if err != nil {
										s.T().Error(err)
										return
									}
								} else {
									err = yaml.ReadYamlString(f.Content, &parsed)
									if err != nil {
										s.T().Error(err)
										return
									}
								}
								parsedJson, err := yaml.WriteJsonString(parsed)
								if err != nil {
									s.T().Error(err)
									return
								}
								assert.Equal(s.T(), string(ef.Parsed.Raw), parsedJson)
							}

							if _, ok := r.FilesByPath[f.Path]; !ok {
								s.T().Errorf("missing path %s", f.Path)
							}
							var keysI []any
							for _, k := range strings.Split(f.Path, "/") {
								keysI = append(keysI, k)
							}
							if _, ok, _ := r.FilesTree.GetNestedObject(keysI...); !ok {
								s.T().Errorf("missing tree item %s", f.Path)
							}
						}
						if !foundFile {
							s.T().Errorf("file not found: %s", ef.Path)
						}
					}

					break
				}
			}
			if !found {
				s.T().Errorf("ref %s not found", er.Ref.Branch)
			}
		}
	}

	// master branch, single file
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Files: []types.GitFile{
					{
						Glob: "test1.yaml",
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "master",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test1.yaml",
						},
						Path: "test1.yaml",
					},
				},
			},
		})
	})

	// master branch, multiple files
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Files: []types.GitFile{
					{
						Glob: "test*.yaml",
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "master",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test1.yaml",
					},
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test2.yaml",
					},
				},
			},
		})
	})

	// master branch, multiple files in subdirs
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Files: []types.GitFile{
					{
						Glob: "dir*/*.yaml",
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "master",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "dir*/*.yaml",
						},
						Path: "dir1/test.yaml",
					},
					{
						File: types.GitFile{
							Glob: "dir*/*.yaml",
						},
						Path: "dir2/test.yaml",
					},
				},
			},
		})
	})

	// b1+b2 branches, no files
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "b.*",
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "b1",
				},
				Files: []types.GitFileMatch{},
			},
			{
				Ref: gittypes.GitRef{
					Branch: "b2",
				},
				Files: []types.GitFileMatch{},
			},
		})
	})

	// b1 branch, multiple files
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "b1",
				},
				Files: []types.GitFile{
					{
						Glob: "test*.yaml",
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "b1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test1.yaml",
					},
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test2.yaml",
					},
				},
			},
		})
	})

	// b1+b2 branch, single file
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "b.*",
				},
				Files: []types.GitFile{
					{
						Glob: "test1.yaml",
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "b1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test1.yaml",
						},
						Path: "test1.yaml",
					},
				},
			},
			{
				Ref: gittypes.GitRef{
					Branch: "b2",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test1.yaml",
						},
						Path: "test1.yaml",
					},
				},
			},
		})
	})

	// b1+b2 branch, multiple files
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "b.*",
				},
				Files: []types.GitFile{
					{
						Glob: "test*.yaml",
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "b1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test1.yaml",
					},
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test2.yaml",
					},
				},
			},
			{
				Ref: gittypes.GitRef{
					Branch: "b2",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test1.yaml",
					},
					{
						File: types.GitFile{
							Glob: "test*.yaml",
						},
						Path: "test2.yaml",
					},
				},
			},
		})
	})

	// b1+b2+c1 branch but with multiple file patterns and parsed yaml
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "b.*",
				},
				Files: []types.GitFile{
					{
						Glob: "test1.yaml",
					},
					{
						Glob:      "test2.yaml",
						ParseYaml: true,
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "b1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test1.yaml",
						},
						Path: "test1.yaml",
					},
					{
						File: types.GitFile{
							Glob: "test2.yaml",
						},
						Path:   "test2.yaml",
						Parsed: &runtime.RawExtension{Raw: []byte("{\"test1\":{\"test2\":143}}")},
					},
				},
			},
			{
				Ref: gittypes.GitRef{
					Branch: "b2",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "test1.yaml",
						},
						Path: "test1.yaml",
					},
					{
						File: types.GitFile{
							Glob: "test2.yaml",
						},
						Path:   "test2.yaml",
						Parsed: &runtime.RawExtension{Raw: []byte("{\"test1\":{\"test2\":243}}")},
					},
				},
			},
		})
	})

	// c1 branch multidoc.yaml
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "c1",
				},
				Files: []types.GitFile{
					{
						Glob:         "multidoc.yaml",
						ParseYaml:    true,
						YamlMultiDoc: true,
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "c1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "multidoc.yaml",
						},
						Path:   "multidoc.yaml",
						Parsed: &runtime.RawExtension{Raw: []byte("[{\"test1\":{\"test2\":1000}},{\"test1\":{\"test2\":2000}}]")},
					},
				},
			},
		})
	})

	// c1 branch rendered.yaml
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "c1",
				},
				Files: []types.GitFile{
					{
						Glob:         "rendered.yaml",
						Render:       true,
						ParseYaml:    true,
						YamlMultiDoc: true,
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "c1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "rendered.yaml",
						},
						Path:    "rendered.yaml",
						Content: "\ntest1:\n  test2: 3\n---\ntest1:\n  test2: 4",
						Parsed:  &runtime.RawExtension{Raw: []byte("[{\"test1\":{\"test2\":3}},{\"test1\":{\"test2\":4}}]")},
					},
				},
			},
		})
	})

	// c1 branch sops.yaml
	s.testVarsLoader(func(vl *VarsLoader, vc *VarsCtx, aws *aws.FakeAwsClientFactory, gcp *gcp.FakeClientFactory) {
		url, _ := gittypes.ParseGitUrl(gs.GitRepoUrl("repo"))
		err = vl.LoadVars(context.TODO(), vc, &types.VarsSource{
			GitFiles: &types.VarsSourceGitFiles{
				Url: *url,
				Ref: &gittypes.GitRef{
					Branch: "c1",
				},
				Files: []types.GitFile{
					{
						Glob:      "sops.yaml",
						ParseYaml: true,
					},
				},
			},
			TargetPath: "my.target",
		}, nil, "")
		assert.NoError(s.T(), err)

		assertResults(vc, "my.target", []types.GitFilesRefMatch{
			{
				Ref: gittypes.GitRef{
					Branch: "c1",
				},
				Files: []types.GitFileMatch{
					{
						File: types.GitFile{
							Glob: "sops.yaml",
						},
						Path:    "sops.yaml",
						Content: "test1:\n    test2: 42\n",
						Parsed:  &runtime.RawExtension{Raw: []byte("{\"test1\":{\"test2\":42}}")},
					},
				},
			},
		})
	})
}
