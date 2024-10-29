package login

import (
	"github.com/mNi-Cloud/cli/internal/pkg/config"
	"github.com/mNi-Cloud/cli/internal/pkg/factory"
	"github.com/mNi-Cloud/cli/internal/pkg/oidc"
	"github.com/urfave/cli/v2"
)

type CommandFactory struct {
}

func (c CommandFactory) Command(name string) *cli.Command {
	return &cli.Command{
		Name: name,
		Action: func(c *cli.Context) error {
			res, err := oidc.GetIdToken(c.Context, c.String("idp-endpoint"), "cloud", "mni-cli")
			if err != nil {
				return err
			}

			conf := &config.Config{
				Token:              *res.IdToken,
				RefreshToken:       *res.RefreshToken,
				TokenExpiry:        *res.Expiry,
				RefreshTokenExpiry: *res.RefreshExpiry,
			}

			err = config.SaveConfig(conf)
			if err != nil {
				return err
			}

			return nil
		},
	}
}

func NewCommandFactory() factory.CommandFactory {
	return &CommandFactory{}
}
