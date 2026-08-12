package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mNi-Cloud/cli/internal/config"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/urfave/cli/v3"
)

func configCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Read and change the contexts of this machine",
		Commands: []*cli.Command{
			{
				Name:   "get-contexts",
				Usage:  "List the contexts",
				Action: deps.GetContexts,
			},
			{
				Name:   "current-context",
				Usage:  "Show the name of the current context",
				Action: deps.CurrentContext,
			},
			{
				Name:      "use-context",
				Usage:     "Make a context the current one",
				ArgsUsage: "<name>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
				Action:    deps.UseContext,
			},
			{
				Name:      "use-tenant",
				Usage:     "Set the tenant of the current context",
				ArgsUsage: "<name>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
				Action:    deps.UseTenant,
			},
			{
				Name:      "delete-context",
				Usage:     "Remove a context and its session",
				ArgsUsage: "<name>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
				Action:    deps.DeleteContext,
			},
		},
	}
}

// GetContexts lists the contexts and marks the current one.
func (d *Deps) GetContexts(_ context.Context, _ *cli.Command) error {
	cfg, err := d.Config()
	if err != nil {
		return err
	}

	table := output.NewWriter(d.Out)
	table.WriteHeader("Current", "Name", "Server", "Tenant")
	for _, target := range cfg.Contexts {
		current := ""
		if target.Name == cfg.CurrentContext {
			current = "*"
		}
		table.WriteRow(current, target.Name, target.Server, target.Tenant)
	}
	return table.Flush()
}

// CurrentContext prints the name of the context commands address by default.
func (d *Deps) CurrentContext(_ context.Context, _ *cli.Command) error {
	cfg, err := d.Config()
	if err != nil {
		return err
	}
	if cfg.CurrentContext == "" {
		return errors.New("no current context is set: run `mni config use-context <name>`")
	}

	fmt.Fprintln(d.Out, cfg.CurrentContext)
	return nil
}

// UseContext points later commands at another context.
func (d *Deps) UseContext(_ context.Context, cmd *cli.Command) error {
	name := cmd.StringArg("name")
	if name == "" {
		return errors.New("mni config use-context needs a context name")
	}

	store, err := d.Store()
	if err != nil {
		return err
	}
	cfg, err := d.Config()
	if err != nil {
		return err
	}
	if err := cfg.SetCurrent(name); err != nil {
		return err
	}
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "Now using context %q.\n", name)
	return nil
}

// UseTenant sets the tenant that namespaced resources are addressed in.
func (d *Deps) UseTenant(_ context.Context, cmd *cli.Command) error {
	name := cmd.StringArg("name")
	if name == "" {
		return errors.New("mni config use-tenant needs a tenant name")
	}

	store, err := d.Store()
	if err != nil {
		return err
	}
	cfg, err := d.Config()
	if err != nil {
		return err
	}
	target, err := d.Context()
	if err != nil {
		return err
	}

	target.Tenant = name
	cfg.Put(target)
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "Context %q now uses tenant %q.\n", target.Name, name)
	return nil
}

// DeleteContext removes a context together with the session behind it.
func (d *Deps) DeleteContext(_ context.Context, cmd *cli.Command) error {
	name := cmd.StringArg("name")
	if name == "" {
		return errors.New("mni config delete-context needs a context name")
	}

	store, err := d.Store()
	if err != nil {
		return err
	}
	cfg, err := d.Config()
	if err != nil {
		return err
	}
	if !cfg.Remove(name) {
		return &config.ContextNotFoundError{Name: name}
	}
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}
	if _, err := store.DeleteCredential(name); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "Removed context %q.\n", name)
	return nil
}
