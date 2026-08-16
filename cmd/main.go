package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/mNi-Cloud/cli/internal/buildinfo"
	"github.com/mNi-Cloud/cli/internal/cli"
)

// version is the release version, stamped in by -ldflags. A build that stamps
// nothing reads its version off the checkout it was made from instead.
var version string

func main() {
	// A command that waits, such as a console offered on a local port, ends on
	// the interrupt of the user instead of being killed halfway through.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cli.NewCommand(buildinfo.Version(version)).Run(ctx, os.Args); err != nil {
		if message := cli.UserFacing(err); message != nil {
			fmt.Fprintln(os.Stderr, "Error:", message)
		}
		os.Exit(cli.ExitCode(err))
	}
}
