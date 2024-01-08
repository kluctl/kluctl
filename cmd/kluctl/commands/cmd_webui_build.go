package commands

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/webui"
	"time"
)

type webuiBuildCmd struct {
	Path        string   `group:"misc" help:"Output path." required:"true"`
	Context     []string `group:"misc" help:"List of kubernetes contexts to use. Defaults to the current context."`
	AllContexts bool     `group:"misc" help:"Use all Kubernetes contexts found in the kubeconfig."`
	MaxResults  int      `group:"misc" help:"Specify the maximum number of results per target." default:"1"`
}

func (cmd *webuiBuildCmd) Help() string {
	return `This command will build the static Kluctl Webui.
`
}

func (cmd *webuiBuildCmd) Run(ctx context.Context) error {
	if !webui.IsWebUiBuildIncluded() {
		return fmt.Errorf("this build of Kluctl does not have the webui embedded")
	}

	stores, _, err := createResultStores(ctx, "", cmd.Context, cmd.AllContexts, false)
	if err != nil {
		return err
	}

	collector := results.NewResultsCollector(ctx, stores)
	collector.Start()

	st := status.Start(ctx, "Collecting summaries")
	defer st.Failed()
	err = collector.WaitForResults(time.Second, time.Second*30)
	if err != nil {
		return err
	}
	st.Success()

	sbw := webui.NewStaticWebuiBuilder(collector, cmd.MaxResults)
	return sbw.Build(ctx, cmd.Path)
}
