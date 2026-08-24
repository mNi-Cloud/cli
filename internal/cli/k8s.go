package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/urfave/cli/v3"
)

const (
	namespaceFlagName     = "namespace"
	labelSelectorFlagName = "label-selector"
	limitFlagName         = "limit"
)

func k8sCommand(deps *Deps) *cli.Command {
	return &cli.Command{
		Name:   "k8s",
		Usage:  "Operate a Kubernetes cluster",
		Before: deps.RequireLogin,
		Commands: []*cli.Command{
			{
				Name:      "kubeconfig",
				Usage:     "Write a short-lived kubeconfig",
				ArgsUsage: "<cluster>",
				Arguments: []cli.Argument{&cli.StringArg{Name: "cluster"}},
				Action:    deps.Kubeconfig,
			},
			{
				Name:      "resources",
				Usage:     "List resource types or read objects in a cluster",
				ArgsUsage: "<cluster> [resource] [name]",
				Arguments: []cli.Argument{
					&cli.StringArg{Name: "cluster"},
					&cli.StringArg{Name: "resource"},
					&cli.StringArg{Name: "name"},
				},
				Flags: []cli.Flag{
					&cli.StringFlag{Name: namespaceFlagName, Aliases: []string{"n"}, Usage: "Kubernetes namespace"},
					&cli.StringFlag{Name: labelSelectorFlagName, Aliases: []string{"l"}, Usage: "Kubernetes label selector"},
					&cli.IntFlag{Name: limitFlagName, Usage: "Maximum objects to return"},
				},
				Action: deps.ClusterResources,
			},
		},
	}
}

func (d *Deps) Kubeconfig(ctx context.Context, cmd *cli.Command) error {
	cluster := cmd.StringArg("cluster")
	if cluster == "" {
		return errors.New("mni k8s kubeconfig needs a cluster name")
	}
	apiClient, err := d.Client()
	if err != nil {
		return err
	}
	tenant, err := d.Tenant()
	if err != nil {
		return err
	}
	raw, _, err := apiClient.Kubeconfig(ctx, tenant, cluster)
	if err != nil {
		return err
	}
	_, err = d.Out.Write(raw)
	return err
}

func (d *Deps) ClusterResources(ctx context.Context, cmd *cli.Command) error {
	cluster := cmd.StringArg("cluster")
	if cluster == "" {
		return errors.New("mni k8s resources needs a cluster name")
	}
	resource, name := cmd.StringArg("resource"), cmd.StringArg("name")
	if name != "" && resource == "" {
		return errors.New("a Kubernetes object name needs a resource type")
	}
	segments := make([]string, 0, 2)
	if resource != "" {
		segments = append(segments, resource)
	}
	if name != "" {
		segments = append(segments, name)
	}
	query := url.Values{}
	if namespace := cmd.String(namespaceFlagName); namespace != "" {
		query.Set(namespaceFlagName, namespace)
	}
	if selector := cmd.String(labelSelectorFlagName); selector != "" {
		query.Set("labelSelector", selector)
	}
	if cmd.IsSet(limitFlagName) {
		limit := cmd.Int(limitFlagName)
		if limit <= 0 {
			return fmt.Errorf("%d is no positive resource limit", limit)
		}
		query.Set(limitFlagName, strconv.Itoa(limit))
	}
	apiClient, err := d.Client()
	if err != nil {
		return err
	}
	tenant, err := d.Tenant()
	if err != nil {
		return err
	}
	raw, err := apiClient.ClusterResources(ctx, tenant, cluster, segments, query)
	if err != nil {
		return err
	}
	return jsonIndent(d.Out, raw)
}
