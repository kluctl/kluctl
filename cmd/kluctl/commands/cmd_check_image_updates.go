package commands

import (
	"github.com/kluctl/kluctl/cmd/kluctl/args"
	"github.com/kluctl/kluctl/pkg/registries"
	"github.com/kluctl/kluctl/pkg/types/k8s"
	"github.com/kluctl/kluctl/pkg/utils"
	"github.com/kluctl/kluctl/pkg/utils/versions"
	log "github.com/sirupsen/logrus"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

type checkImageUpdatesCmd struct {
	args.ProjectFlags
	args.TargetFlags
}

func (cmd *checkImageUpdatesCmd) Help() string {
	return `This is based on a best effort approach and might give many false-positives.`
}

func (cmd *checkImageUpdatesCmd) Run() error {
	var renderedImages map[k8s.ObjectRef][]string
	ptArgs := projectTargetCommandArgs{
		projectFlags: cmd.ProjectFlags,
		targetFlags:  cmd.TargetFlags,
	}
	err := withProjectCommandContext(ptArgs, func(ctx *commandCtx) error {
		renderedImages = ctx.deploymentCollection.FindRenderedImages()
		return nil
	})
	if err != nil {
		return err
	}

	wg := utils.NewWorkerPoolWithErrors(8)
	defer wg.StopWait(false)

	imageTags := make(map[string]interface{})
	var mutex sync.Mutex

	for _, images := range renderedImages {
		for _, image := range images {
			s := strings.SplitN(image, ":", 2)
			if len(s) == 1 {
				continue
			}
			repo := s[0]
			if _, ok := imageTags[repo]; !ok {
				wg.Submit(func() error {
					tags, err := registries.ListImageTags(repo)
					mutex.Lock()
					defer mutex.Unlock()
					if err != nil {
						imageTags[repo] = err
					} else {
						imageTags[repo] = tags
					}
					return nil
				})
			}
		}
	}
	err = wg.StopWait(false)
	if err != nil {
		return err
	}

	prefixPattern := regexp.MustCompile("^([a-zA-Z]+[a-zA-Z-_.]*)")
	suffixPattern := regexp.MustCompile("([-][a-zA-Z]+[a-zA-Z-_.]*)$")

	var table utils.PrettyTable
	table.AddRow("Object", "Image", "Old", "New")

	for ref, images := range renderedImages {
		for _, image := range images {
			s := strings.SplitN(image, ":", 2)
			if len(s) == 1 {
				log.Warningf("%s: Ignoring image %s as it doesn't specify a tag", ref.String(), image)
				continue
			}
			repo := s[0]
			curTag := s[1]
			repoTags, _ := imageTags[repo].([]string)
			err, _ := imageTags[repo].(error)
			if err != nil {
				log.Warningf("%s: Failed to list tags for %s. %v", ref.String(), repo, err)
				continue
			}

			prefix := prefixPattern.FindString(curTag)
			suffix := suffixPattern.FindString(curTag)
			hasDot := strings.Index(curTag, ".") != -1

			var filteredTags []string
			for _, tag := range repoTags {
				hasDot2 := strings.Index(tag, ".") != -1
				if hasDot != hasDot2 {
					continue
				}
				if prefix != "" && !strings.HasPrefix(tag, prefix) {
					continue
				}
				if suffix != "" && !strings.HasSuffix(tag, suffix) {
					continue
				}
				filteredTags = append(filteredTags, tag)
			}
			doKey := func(tag string) versions.LooseVersion {
				if prefix != "" {
					tag = tag[len(prefix):]
				}
				if suffix != "" {
					tag = tag[:len(tag)-len(suffix)]
				}
				return versions.LooseVersion(tag)
			}
			sort.SliceStable(filteredTags, func(i, j int) bool {
				a := doKey(filteredTags[i])
				b := doKey(filteredTags[j])
				return a.Less(b, true)
			})
			latestTag := filteredTags[len(filteredTags)-1]

			if latestTag != curTag {
				table.AddRow(ref.String(), repo, curTag, latestTag)
			}
		}
	}

	table.SortRows(1)
	_, _ = os.Stdout.WriteString(table.Render([]int{60}))
	return nil
}
