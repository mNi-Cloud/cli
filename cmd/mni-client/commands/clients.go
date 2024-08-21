package commands

import (
	mni_bs "github.com/mNi-Cloud/backend/bs/pkg/client"
	mni_ctr "github.com/mNi-Cloud/backend/ctr/pkg/client"
	mni_vm "github.com/mNi-Cloud/backend/vm/pkg/client"
	mni_vpc "github.com/mNi-Cloud/backend/vpc/pkg/client"
	"github.com/urfave/cli/v2"
)

func NewVpcClient(ctx *cli.Context) (*mni_vpc.Client, error) {
	return mni_vpc.NewClient(ctx.String("vpc-endpoint"))
}

func NewVmClient(ctx *cli.Context) (*mni_vm.Client, error) {
	return mni_vm.NewClient(ctx.String("vm-endpoint"))
}

func NewBsClient(ctx *cli.Context) (*mni_bs.Client, error) {
	return mni_bs.NewClient(ctx.String("bs-endpoint"))
}

func NewCtrClient(ctx *cli.Context) (*mni_ctr.Client, error) {
	return mni_ctr.NewClient(ctx.String("ctr-endpoint"))
}
