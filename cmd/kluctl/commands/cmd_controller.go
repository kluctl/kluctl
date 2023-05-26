package commands

type controllerCmd struct {
	Install controllerInstallCmd `cmd:"" help:"Install the Kluctl controller"`
	Run_    controllerRunCmd     `cmd:"run" help:"Run the Kluctl controller"`
}
