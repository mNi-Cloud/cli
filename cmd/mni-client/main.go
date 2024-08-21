package main

import (
	"fmt"
	"os"

	"github.com/mNi-Cloud/cli/cmd/mni-client/app"
)

func main() {
	app := app.New()

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
