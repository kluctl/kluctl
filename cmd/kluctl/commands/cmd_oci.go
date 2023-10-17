package commands

type ociCmd struct {
	Push ociPushCmd `cmd:"" help:"Push to an oci repository"`
}
