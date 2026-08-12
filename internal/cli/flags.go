package cli

import (
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

// outputFlagName is the flag that picks the output format of a read.
const outputFlagName = "output"

// outputFlag builds the format flag every command that reads resources takes.
func outputFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    outputFlagName,
		Aliases: []string{"o"},
		Usage:   "Output format (table|json|yaml|jsonpath=<path>)",
		Value:   output.FormatTable,
	}
}

// yesFlag builds the flag that answers the confirmation of a command that
// destroys something.
func yesFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:    yesFlagName,
		Aliases: []string{"y"},
		Usage:   "Delete without asking for confirmation",
	}
}
