package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/mNi-Cloud/backend/auth/pkg/kc"
	mni_bs "github.com/mNi-Cloud/backend/bs/pkg/client"
	mni_ctr "github.com/mNi-Cloud/backend/ctr/pkg/client"
	mni_vm "github.com/mNi-Cloud/backend/vm/pkg/client"
	mni_vpc "github.com/mNi-Cloud/backend/vpc/pkg/client"
	"github.com/mNi-Cloud/cli/internal/pkg/config"
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

func TokenFunc() cli.BeforeFunc {
	return func(c *cli.Context) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			if os.IsNotExist(err) {
				return cli.Exit("Please login by running 'mni login' command", 1)
			}
			return cli.Exit("Failed to load config: "+err.Error(), 1)
		}

		if cfg.TokenExpiry.After(time.Now()) {
			c.Set("token", cfg.Token)
			return nil
		}

		if cfg.RefreshTokenExpiry.After(time.Now()) {
			client, err := kc.NewClient(c.Context, c.String("idp-endpoint"), "cloud")
			if err != nil {
				return cli.Exit("Failed to create keycloak client: "+err.Error(), 1)
			}
			res, err := client.RefreshToken(c.Context, "mni-cli", cfg.RefreshToken)
			if err != nil {
				return cli.Exit("Failed to refresh token: "+err.Error(), 1)
			}

			expiry := time.Now().Add(time.Duration(res.ExpiresIn) * time.Second)
			refreshExpiry := time.Now().Add(time.Duration(res.RefreshExpiresIn) * time.Second)

			cfg := config.Config{
				Token:              res.IdToken,
				RefreshToken:       res.RefreshToken,
				TokenExpiry:        expiry,
				RefreshTokenExpiry: refreshExpiry,
			}

			err = config.SaveConfig(&cfg)
			if err != nil {
				fmt.Println("Failed to save token")
			}
			c.Set("token", res.IdToken)
			return nil
		}

		return cli.Exit("Please login by running 'mni login' command", 1)
	}
}
