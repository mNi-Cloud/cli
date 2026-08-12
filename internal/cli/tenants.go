package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/output"
	"github.com/mNi-Cloud/cli/internal/unstructured"
	"github.com/urfave/cli/v3"
)

const (
	displayNameFlagName = "display-name"
	descriptionFlagName = "description"
	roleFlagName        = "role"
)

func tenantsCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:    "tenants",
		Aliases: []string{"tenant"},
		Usage:   "Read and change the tenants you take part in",
		Flags:   []cli.Flag{outputFlag()},
		Before:  deps.RequireLogin,
		Action:  deps.ListTenants,
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List the tenants you take part in",
				Flags:  []cli.Flag{outputFlag()},
				Action: deps.ListTenants,
			},
			{
				Name:      "create",
				Usage:     "Open a tenant owned by you",
				ArgsUsage: "<name>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
				Flags: []cli.Flag{
					&cli.StringFlag{Name: displayNameFlagName, Usage: "Name to show instead of the one addressed by"},
					&cli.StringFlag{Name: descriptionFlagName, Usage: "What the tenant is for"},
				},
				Action: deps.CreateTenant,
			},
			{
				Name:      "delete",
				Usage:     "Remove a tenant and everything inside it",
				ArgsUsage: "<name>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "name"}},
				Flags:     []cli.Flag{yesFlag()},
				Action:    deps.DeleteTenant,
			},
			{
				Name:      "members",
				Usage:     "List the members of a tenant",
				ArgsUsage: "<tenant>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "tenant"}},
				Flags:     []cli.Flag{outputFlag()},
				Action:    deps.TenantMembers,
			},
			{
				Name:      "add-member",
				Usage:     "Let a user into a tenant, by the name they log in with",
				ArgsUsage: "<tenant> <username>",
				Arguments: []cli.Argument{
					&cli.StringArg{Name: "tenant"},
					&cli.StringArg{Name: "user"},
				},
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: roleFlagName, Usage: "Role to give the member (repeat for several)"},
				},
				Action: deps.AddTenantMember,
			},
			{
				Name:      "remove-member",
				Usage:     "Take a user out of a tenant, by the id `mni tenants members` lists",
				ArgsUsage: "<tenant> <user-id>",
				Arguments: []cli.Argument{
					&cli.StringArg{Name: "tenant"},
					&cli.StringArg{Name: "user"},
				},
				Action: deps.RemoveTenantMember,
			},
		},
	}
}

// ListTenants prints the tenants the caller owns or is a member of.
func (d *Deps) ListTenants(ctx context.Context, cmd *cli.Command) error {
	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	tenants, err := apiClient.Tenants(ctx)
	if err != nil {
		return err
	}

	format := cmd.String(outputFlagName)
	if format == output.FormatTable {
		return d.Printer().PrintTenants(tenants)
	}

	list, err := unstructured.ListFrom(tenants)
	if err != nil {
		return err
	}
	return d.Printer().Print(list, format)
}

// CreateTenant opens a tenant with the caller as its owner.
func (d *Deps) CreateTenant(ctx context.Context, cmd *cli.Command) error {
	name := cmd.StringArg("name")
	if name == "" {
		return errors.New("mni tenants create needs a name (usage: mni tenants create <name>)")
	}

	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	created, err := apiClient.CreateTenant(ctx, api.NewTenant{
		Name:        name,
		DisplayName: cmd.String(displayNameFlagName),
		Description: cmd.String(descriptionFlagName),
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "tenants/%s created\n", created.Name)
	return nil
}

// DeleteTenant removes a tenant with everything that lives in it.
func (d *Deps) DeleteTenant(ctx context.Context, cmd *cli.Command) error {
	name := cmd.StringArg("name")
	if name == "" {
		return errors.New("mni tenants delete needs a name (usage: mni tenants delete <name>)")
	}

	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	if !cmd.Bool(yesFlagName) {
		if !d.Interactive {
			return notATerminal(fmt.Sprintf("whether to delete tenants/%s", name))
		}

		fmt.Fprintf(d.Out, "Deleting tenants/%s deletes every resource inside it.\n", name)
		allowed, err := d.Confirm(fmt.Sprintf("Delete tenants/%s?", name))
		if err != nil {
			return err
		}
		if !allowed {
			fmt.Fprintf(d.Out, "tenants/%s was not deleted.\n", name)
			return nil
		}
	}

	if err := apiClient.DeleteTenant(ctx, name); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "tenants/%s deleted\n", name)
	return nil
}

// TenantMembers prints the members a tenant has beside its owner.
func (d *Deps) TenantMembers(ctx context.Context, cmd *cli.Command) error {
	tenant := cmd.StringArg("tenant")
	if tenant == "" {
		return errors.New("mni tenants members needs a tenant (usage: mni tenants members <tenant>)")
	}

	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	members, err := apiClient.Members(ctx, tenant)
	if err != nil {
		return err
	}

	format := cmd.String(outputFlagName)
	if format == output.FormatTable {
		return d.Printer().PrintMembers(members)
	}

	list, err := unstructured.ListFrom(members)
	if err != nil {
		return err
	}
	return d.Printer().Print(list, format)
}

// AddTenantMember lets another user work in a tenant.
func (d *Deps) AddTenantMember(ctx context.Context, cmd *cli.Command) error {
	tenant := cmd.StringArg("tenant")
	user := cmd.StringArg("user")
	if tenant == "" || user == "" {
		return errors.New("mni tenants add-member needs a tenant and a user (usage: mni tenants add-member <tenant> <user>)")
	}

	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	if _, err := apiClient.AddMember(ctx, tenant, api.NewMember{
		Username: user,
		Roles:    cmd.StringSlice(roleFlagName),
	}); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "%s is now a member of tenants/%s\n", user, tenant)
	return nil
}

// RemoveTenantMember takes a user out of a tenant.
func (d *Deps) RemoveTenantMember(ctx context.Context, cmd *cli.Command) error {
	tenant := cmd.StringArg("tenant")
	user := cmd.StringArg("user")
	if tenant == "" || user == "" {
		return errors.New("mni tenants remove-member needs a tenant and a user (usage: mni tenants remove-member <tenant> <user>)")
	}

	apiClient, err := d.Client()
	if err != nil {
		return err
	}

	if _, err := apiClient.RemoveMember(ctx, tenant, user); err != nil {
		return err
	}

	fmt.Fprintf(d.Out, "%s is no longer a member of tenants/%s\n", user, tenant)
	return nil
}
