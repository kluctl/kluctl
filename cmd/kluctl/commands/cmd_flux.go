package commands

type fluxCmd struct {
	Reconcile fluxReconcileCmd `cmd:"" help:"Reconcile KluctlDeployment"`
	Suspend   fluxSuspendCmd   `cmd:"" help:"Suspend KluctlDeployment"`
	Resume    fluxResumeCmd    `cmd:"" help:"Resume KluctlDeployment"`
}
