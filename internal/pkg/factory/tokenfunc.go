package factory

import (
	"fmt"
	"os"
	"time"

	"github.com/mNi-Cloud/backend/auth/pkg/kc"
	"github.com/mNi-Cloud/cli/internal/pkg/config"
	"github.com/urfave/cli/v2"
)

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
			err := c.Set("token", cfg.Token)
			if err != nil {
				return err
			}
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
			err = c.Set("token", res.IdToken)
			if err != nil {
				return err
			}
			return nil
		}

		return cli.Exit("Please login by running 'mni login' command", 1)
	}
}
