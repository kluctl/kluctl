package commands

type controllerCmd struct {
	Run_ controllerRunCmd `cmd:"run" help:"Run the Kluctl controller"`
}
