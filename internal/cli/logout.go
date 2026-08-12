package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func logoutCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:   "logout",
		Usage:  "Forget the session of the current context",
		Action: deps.Logout,
	}
}

// Logout drops the tokens of a context, leaving the context itself in place.
func (d *Deps) Logout(_ context.Context, _ *cli.Command) error {
	store, err := d.Store()
	if err != nil {
		return err
	}
	target, err := d.Context()
	if err != nil {
		return err
	}

	removed, err := store.DeleteCredential(target.Name)
	if err != nil {
		return err
	}
	if !removed {
		fmt.Fprintf(d.Out, "There is no session to forget for context %q.\n", target.Name)
		return nil
	}

	fmt.Fprintf(d.Out, "Signed out of context %q.\n", target.Name)
	return nil
}
