package factory

import "github.com/urfave/cli/v2"

type (
	CommandFactory interface {
		Command(name string) *cli.Command
	}
)
